gofront: an interpreter-front end for Golang
=======

Go, if it only had a decent REPL, could be a great
language for interactive, exploratory data analysis.

Go's big advantage over python, R, and Matlab is that
it has good type checking, good compiled performance,
and excellent multicore and distributed support.

`gofront` aims to provide a library for many
interactive backends to be written against.
`gofront` will provide the REPL and do the
initial lexical scanning, parsing, and type
checking. `gofront` will then pass the
AST to a backend for codegen and/or immediate
interpretation.

Considering possible backends for a
reference implementation,
I compared node.js, chez scheme, otto,
gopher-lua, and luajit.

Luajit in particular is an amazing
backend to target. In our quick and
dirty 500x500 matrix multiplication
benchmark, luajit *beat even statically compiled go*
code by a factor 3x.

Luajit (for speed) or gopher-lua (for
ease of embedding) will probably be the
choice(s) for reference backend
implementation.


Author: Jason E. Aten, Ph.D.

License: 3-clause BSD. See LICENSE.

Credits: some code here is dervied from the Go gc compiler
and from Richard Musiol's excellent Gopherjs project.
