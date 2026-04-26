here is the original plan file used to start this project:

- we are going to build a minimal interactive shell based on https://github.com/mvdan/sh
- it should execute shell scripts, accept shell code to run (-c), or be run as a repl.
- we want to optimize for least amount of code to make iteration in the future easier.
- just as context, we may eventually want to build our own version of mvdan/sh but for plan 9's rc
- so this is just a minimal shell that wanix can use in the meantime.
- it will be compiled with GOOS=js GOARCH=wasm and run in wanix.
- this means we can rely on the stdlib os operations for most things except exec.
- wiring up pipes is a nice to have but should probably be done last after we understand
  the i/o model with terminals in wanix.
- a lot of the builtins mvdan/sh implements assume posix. we cant support all of them.
- we do want basic cp, rm, etc which seems like mvdan/sh uses u-root for (in moreinterp)

how to run processes in wanix:
- you can see an incomplete but latest iteration of the task api in elements/task.js
- an older version used to run from inside a vm can be found in shell/wexec
- basic idea:
- allocate a task by reading #task/new/auto to get a resource id
- this is used to get the task path: #task/$rid
- now the cmd, env, dir files can be written to to set up the task
- stdio is wired up using bindings on the task namespace:
  - #task/$rid/fd/0,1,2
- start after setup: write "start" to #task/$rid/ctl
- exit file will have the exit code when completed

terminals/pty in wanix:
- wanix doesnt have a normal pty system. it has "terms" which work like this:
  - allocate with #term/new to get rid
  - #term/$rid/data is the "terminal" side to read and write from
  - #term/$rid/program is the "program" side
  - #term/$rid/winch is a broadcast file for window changes:
    - readers subscribe to writes to the file
    - writer writes are broadcast to all readers
- a task wired up to a term works like this:
  - #task/$rid/fd/0,1,2 are all bindings to #term/$rid/program
- i think subprocesses can have stdout,stderr (1,2) bound to the same program file
- but i think stdin (0) needs its own pipe managed by the parent process
  that owns the terminal since it already owns stdin from it

decision log:
- mvp boundaries: what is explicitly in/out for v1 (eg no globbing, command substitution, functions, background jobs)
    - support simple command execution, sequencing, env vars, scripts, and -c; defer pipelines, command substitution, background jobs, shell functions, job control, and advanced expansions.

- builtins for day one: which must be native in the shell (eg cd, pwd, echo, exit, export, unset, cp, rm, mv, mkdir, cat)
    - builtins for day one: implement only `cd`, `pwd`, `echo`, `exit`, `export`, `unset`

- cp/rm implementation source: reuse existing mvdan/u-root implementations or create wanix-native builtins
    - wanix doesnt have equivalent cp or rm as commands. we can use u-root implementations for now.

- external command lookup semantics: PATH behavior, relative vs absolute paths, and executable checks in wanix
    - lets do Plan9 PATH behavior which i mean is mostly like POSIX, but you can use subpaths of PATH:
        - PATH=/bin then auth/foo should work if /bin/auth/foo exists
    - we want to support relative and absolute paths, which should be taken care of by wanix (so POSIX)
    - lets check the executable bit but for now just warn if its not set

- stdio wiring contract: confirm whether stdin must be parent-managed while stdout/stderr can share #term/$rid/program
    - yes parent-owned stdin pipe feeding child fd 0; bind child fd 1 and fd 2 to `#term/$rid/program`.

- repl ux scope: raw repl first or line editing/history/completion in v1
    - raw repl first (read/eval/print loop), no history/completion/editing in v1.

- mode parity: should script file mode, -c mode, and repl non-interactive execution share identical env/exit semantics
    - keep script mode, `-c`, and repl non-interactive evaluation behavior consistent for env setup and exit status.

- exit status behavior: decide expected compatibility for builtin failures and future set -e/pipeline semantics
    - use simple last-command exit code semantics; no `set -e` or pipeline status behavior until pipeline support exists.

- terminal resize propagation: define how #term/$rid/winch events map to running programs
    - we dont have signals yet, so stub and comment where we would hook in but otherwise leave this out

- compatibility target: prioritize posix shell behavior vs intentionally rc-flavored interim behavior
    - we will start posix oriented since mvdan/sh is made for that, but make notes on where behavior would change for more rc-style compat
