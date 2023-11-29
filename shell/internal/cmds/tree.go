package cmds

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"tractor.dev/toolkit-go/engine/cli"
	. "tractor.dev/wanix/shell/internal/sharedutil"
)

type treeNode struct {
	name                   string
	parent, sibling, child *treeNode
}

func (t *treeNode) addSibling(sib *treeNode) {
	node := t
	for ; node.sibling != nil; node = node.sibling {
	}
	node.sibling = sib
}

func (t *treeNode) addChild(child *treeNode) {
	if t.child != nil {
		t.child.addSibling(child)
	} else {
		t.child = child
	}
}

func (t *treeNode) populate(dirpath string) error {
	if t.parent != nil && (filepath.Base(dirpath) == ".git" || dirpath == "/sys/dev") {
		return nil
	}

	entries, err := os.ReadDir(dirpath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		child := &treeNode{name: entry.Name(), parent: t}
		t.addChild(child)

		if entry.IsDir() {
			// TODO: possible stack overflow
			err := child.populate(filepath.Join(dirpath, entry.Name()))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *treeNode) render(w io.Writer) {
	if t.parent == nil {
		w.Write([]byte(fmt.Sprintf("%s\n", t.name)))
	} else {
		// Stack of whether t's ancestors have a sibling,
		// excluding the root node.
		// TODO: We're creating a stack for each invocation, but
		// most nodes will have siblings with the exact same stack.
		// There's probably a more optimal way of doing this.
		siblingStack := make([]bool, 0)
		for p := t.parent; p != nil && p.parent != nil; p = p.parent {
			siblingStack = append(siblingStack, p.sibling != nil)
		}

		for i := len(siblingStack) - 1; i >= 0; i-- {
			if siblingStack[i] {
				w.Write([]byte("│   "))
			} else {
				w.Write([]byte("    "))
			}
		}

		if t.sibling != nil {
			w.Write([]byte("├── "))
		} else {
			w.Write([]byte("└── "))
		}

		w.Write([]byte(t.name + "\n"))
	}

	for c := t.child; c != nil; c = c.sibling {
		// TODO: possible stack overflow
		c.render(w)
	}
}

func TreeCmd() *cli.Command {
	return &cli.Command{
		Usage: "tree [directory]",
		Args:  cli.MaxArgs(1),
		Short: "Prints a file tree rooted at the given directory, or the working directory if none is specified.",
		Run: func(ctx *cli.Context, args []string) {
			var dir string
			if len(args) > 0 {
				dir = AbsPath(args[0])
			} else {
				var err error
				dir, err = os.Getwd()
				if CheckErr(ctx, err) {
					return
				}
			}

			finishedTreeGen := false
			go func() {
				time.Sleep(time.Second)
				if !finishedTreeGen {
					ctx.Errout().Write([]byte("Generating file tree. This may take a moment...\n"))
				}
			}()

			root := &treeNode{name: filepath.Base(dir)}
			treeErr := root.populate(dir)
			finishedTreeGen = true

			// render what we have then show the error, if any.
			root.render(ctx)
			CheckErr(ctx, treeErr)
		},
	}
}
