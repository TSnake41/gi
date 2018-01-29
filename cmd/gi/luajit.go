package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gijit/gi/pkg/compiler"
	"github.com/gijit/gi/pkg/front"
	"github.com/gijit/gi/pkg/verb"
	//luajit "github.com/glycerine/golua/lua"
)

func (cfg *GIConfig) LuajitMain() {

	vmCfg := compiler.NewVmConfig()
	vmCfg.PreludePath = cfg.PreludePath
	vmCfg.Quiet = cfg.Quiet
	vmCfg.NotTestMode = !cfg.IsTestMode
	vm, err := compiler.NewLuaVmWithPrelude(vmCfg)
	panicOn(err)
	defer vm.Close()
	inc := compiler.NewIncrState(vm, vmCfg)

	var t0, t1 time.Time
	var history []string
	home := os.Getenv("HOME")
	var histFn string
	var histFile *os.File
	if home != "" {
		histFn = home + string(os.PathSeparator) + ".gi.hist"

		// open and close once to read back history
		history, err = readHistory(histFn)
		panicOn(err)

		// re-open for append new history
		histFile, err = os.OpenFile(histFn,
			os.O_WRONLY|os.O_CREATE|os.O_APPEND|os.O_SYNC,
			0600)
		panicOn(err)
	}

	_ = inc
	reader := bufio.NewReader(os.Stdin)
	goPrompt := "gi> "
	goMorePrompt := ">>>    "
	luaPrompt := "raw luajit gi> "
	prompt := goPrompt
	if cfg.RawLua {
		prompt = luaPrompt
	}
	prevSrc := ""
	var by []byte

	for {
		fmt.Printf(prompt)
		by, err = reader.ReadBytes('\n')
		if err == io.EOF {
			if len(by) > 0 {
				// process bytes first,
				// return next time.
				err = nil
			} else {
				fmt.Printf("[EOF]\n")
				return
			}
		}
		panicOn(err)
		use := string(by)
		src := use
		cmd := bytes.TrimSpace(by)
		low := string(bytes.ToLower(cmd))
		if len(low) > 1 && low[0] == ':' {
			if low[:2] == "::" {
				// likely the start of a lua label for a goto, not a special : command.
				goto notColonCmd
			}
			if low[1] >= '0' && low[1] <= '9' {
				num, err := strconv.Atoi(low[1:])
				if err != nil {
					fmt.Printf("bad history request, must be digits only after ':'.\n")
					continue
				}
				if num < 1 || num > len(history) {
					fmt.Printf("bad history request, out of range.\n")
					continue
				}
				fmt.Printf("replay history %03d:\n", num)
				src = history[num-1]
				fmt.Printf("%s\n", src)
			}
		}
		switch low {
		case ":ast":
			inc.PrintAST = true
			continue
		case ":noast":
			inc.PrintAST = false
			continue
		case ":q":
			fmt.Printf("quiet mode\n")
			verb.Verbose = false
			verb.VerboseVerbose = false
			continue
		case ":v":
			fmt.Printf("verbose mode.\n")
			verb.Verbose = true
			verb.VerboseVerbose = false
			continue
		case ":vv":
			fmt.Printf("very verbose mode.\n")
			verb.Verbose = true
			verb.VerboseVerbose = true
			continue
		case ":h":
			fmt.Printf("history:\n")
			newline := "\n"
			for i, h := range history {
				lenh := len(h)
				switch {
				case lenh == 0:
					newline = "\n"
				case h[lenh-1] == '\n':
					newline = ""
				default:
					newline = "\n"
				}
				fmt.Printf("%03d: %s%s", i+1, h, newline)
			}
			fmt.Printf("\n")
			continue
		case ":raw", ":r":
			cfg.RawLua = true
			prompt = luaPrompt
			fmt.Printf("Raw LuaJIT language mode.\n")
			continue
		case ":go":
			cfg.RawLua = false
			prompt = goPrompt
			fmt.Printf("Go language mode.\n")
			continue
		case ":prelude", ":reload":
			fmt.Printf("Reloading prelude...\n")

			files, err := compiler.FetchPreludeFilenames(cfg.PreludePath, cfg.Quiet)
			if err != nil {
				fmt.Printf("error during prelude reload: '%v'", err)
				continue
			}
			err = compiler.LuaDoFiles(vm, files)
			if err != nil {
				fmt.Printf("error during prelude reload: '%v'", err)
			}
			continue
		case ":help", ":?":
			fmt.Printf(`
======================
gi: a go interpreter
https://github.com/gijit/gi
command prompt help: 
simply type Go expressions or statements
directly at the prompt, or use one of 
these special commands:
======================
 :v          turns on verbose debug printing
 :vv         turns on very verbose printing
 :q          quiets the debug prints (default)
 :r or :raw  change to raw-luajit entry mode
 :go         change back from raw mode to Go mode
 :ast        print the Go AST prior to translation
 :noast      stop printing the Go AST
 :do <path>  run dofile(path) on a .lua file
 :?          show this help (:help does the same)
 :h          show command history
 :30         replay command number 30 from history
 ctrl-d to exit
`)
			continue
		}

		if strings.HasPrefix(low, ":do") {
			files := strings.TrimSpace(low[3:])
			splt := strings.Split(files, ",")
			var final, show []string
			for i := range splt {
				tmp := strings.TrimSpace(splt[i])
				home := os.Getenv("HOME")
				if home != "" {
					tmp = strings.Replace(tmp, "~/", home+"/", 1)
				}
				if len(tmp) > 0 {
					final = append(final, tmp)
					show = append(show, strconv.Quote(tmp))
				}
			}
			if len(final) > 0 {
				fmt.Printf("running dofile(%s)\n", strings.Join(show, ","))
				err := compiler.LuaDoFiles(vm, final)
				if err != nil {
					fmt.Printf("error during dofile(): '%v'\n", err)
				}
			} else {
				fmt.Printf("nothing to do.\n")
			}
			continue
		}

	notColonCmd:
		isContinuation := len(prevSrc) > 0
		if !cfg.RawLua {
			if isContinuation {
				src = prevSrc + "\n" + src
			}
			//fmt.Printf("src = '%s'\n", src)
			//fmt.Printf("prevSrc = '%s'\n", prevSrc)

			eof, syntaxErr, empty := front.TopLevelParseGoSource([]byte(src))
			if empty {
				prevSrc = ""
				continue
			}
			//fmt.Printf("eof = %v, syntaxErr = %v\n", eof, syntaxErr)
			if eof && !syntaxErr {
				prompt = goMorePrompt
				// get another line of input
				prevSrc = src
				continue
			}
			prevSrc = ""

			prompt = goPrompt
			translation, err := translateAndCatchPanic(inc, []byte(src))
			if err != nil {
				fmt.Printf("oops: '%v' on input '%s'\n", err, strings.TrimSpace(string(src)))
				translation = "\n"
				// still write, so we get another prompt
			} else {
				p("got translation of line from Go into lua: '%s'\n", strings.TrimSpace(string(translation)))
			}
			use = translation

		} else {
			use += "\n"
		}

		p("sending use='%v'\n", use)
		history = append(history, src)
		if histFile != nil {
			fmt.Fprintf(histFile, src)
			histFile.Sync()
		}
		t0 = time.Now()
		// 	loadstring: returns 0 if there are no errors or 1 in case of errors.
		interr := vm.LoadString(use)
		if interr != 0 {
			fmt.Printf("error from Lua vm.LoadString(): supplied lua with: '%s'\nlua stack:\n", use[:len(use)-1])
			compiler.DumpLuaStack(vm)
			vm.Pop(1)
			continue
		}
		err = vm.Call(0, 0)
		if err != nil {
			fmt.Printf("error from Lua vm.Call(0,0): '%v'. supplied lua with: '%s'\nlua stack:\n", err, use[:len(use)-1])
			compiler.DumpLuaStack(vm)
			vm.Pop(1)
			continue
		}
		t1 = time.Now()
		// jea debug:
		//compiler.DumpLuaStack(vm)
		fmt.Printf("\n")
		reader.Reset(os.Stdin)
		fmt.Printf("elapsed: '%v'\n", t1.Sub(t0))
	}
}

/*
func DumpLuaStack(L *luajit.State) {
	var top int

	top = L.GetTop()
	for i := 1; i <= top; i++ {
		t := L.Type(i)
		switch t {
		case luajit.Tstring:
			fmt.Println("String : \t", L.Tostring(i))
		case luajit.Tboolean:
			fmt.Println("Bool : \t\t", L.Toboolean(i))
		case luajit.Tnumber:
			fmt.Println("Number : \t", L.Tonumber(i))
		default:
			fmt.Println("Type : \t\t", L.Typename(i))
		}
	}
	print("\n")
}

func DumpLuaStackAsString(L *luajit.State) string {
	var top int
	s := ""
	top = L.Gettop()
	for i := 1; i <= top; i++ {
		t := L.Type(i)
		switch t {
		case luajit.Tstring:
			s += fmt.Sprintf("String : \t%v", L.Tostring(i))
		case luajit.Tboolean:
			s += fmt.Sprintf("Bool : \t\t%v", L.Toboolean(i))
		case luajit.Tnumber:
			s += fmt.Sprintf("Number : \t%v", L.Tonumber(i))
		default:
			s += fmt.Sprintf("Type : \t\t%v", L.Typename(i))
		}
	}
	return s
}
*/
