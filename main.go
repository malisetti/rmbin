package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/juju/fslock"
)

const trashConfPath = "/home/s/.trashconf.json"
const trashCleanTTL = 60

// 7 * 24 * 60 * 60

type trash struct {
	Dir     string                 `json:"dir"`
	Pathmap map[string]interface{} `json:"pathmap"`
	TTL     int                    `json:"ttl"`
}

func newTrash(p string, ttl int) *trash {
	return &trash{
		Dir:     p,
		Pathmap: make(map[string]interface{}),
		TTL:     ttl,
	}
}

func (t *trash) put(cdir string, xs ...string) error {
	for _, x := range xs {
		rp, err := filepath.Rel(cdir, filepath.Dir(x))
		if err != nil {
			rp = cdir
		}
		ap, err := filepath.Abs(rp)
		if err != nil {
			ap = rp
		}

		tx0 := filepath.Join(t.Dir, ap)
		tx := filepath.Join(tx0, filepath.Base(x))
		tx1 := filepath.Join(ap, filepath.Base(x))

		_ = os.MkdirAll(tx0, 0755)
		err = os.Rename(tx1, tx)
		if err != nil {
			return err
		}
		t.Pathmap[tx1] = nil
	}

	return nil
}

func (t *trash) restore(cdir string, xs ...string) error {
	for _, x := range xs {
		rp, err := filepath.Rel(cdir, x)
		if err != nil {
			rp = cdir
		}
		ap, err := filepath.Abs(rp)
		if err != nil {
			ap = rp
		}

		tx1 := filepath.Join(ap, x)

		if _, ok := t.Pathmap[tx1]; ok {
			err := os.Rename(filepath.Join(t.Dir, tx1), tx1)
			if err != nil {
				return err
			}
			delete(t.Pathmap, tx1)
			tx1 = filepath.Join(t.Dir, tx1)
			for {
				tx1 = filepath.Dir(tx1)
				if tx1 == t.Dir {
					break
				}
				bf, err := os.Open(tx1)
				if err != nil {
					return err
				}
				defer bf.Close()
				ns, _ := bf.Readdirnames(1)
				if len(ns) <= 1 {
					return os.Remove(tx1)
				}
			}
		}
	}
	return nil
}

func (t *trash) list() {
	for k, _ := range t.Pathmap {
		fmt.Println(k)
	}
}

func (t *trash) clean() error {
	return filepath.WalkDir(t.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && info.ModTime().Before(time.Now().Add(time.Duration(t.TTL))) {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	})
}

func main() {
	var t *trash = &trash{}
	var err error

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}

	switch os.Args[1] {
	case "put", "p", "restore", "r", "ls", "gc":
	default:
		fmt.Fprintf(os.Stderr, "%s\n", "put, p, restore, r, ls, gc are valid cmds")
		os.Exit(1)
	}

	conff, err := os.ReadFile(trashConfPath)
	if err == nil {
		err = json.Unmarshal(conff, &t)
	}

	if err != nil {
		td, err := ioutil.TempDir("", "*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
		t = newTrash(td, trashCleanTTL)
	}
	if t.Pathmap == nil {
		t.Pathmap = make(map[string]interface{})
	}
	t.TTL = trashCleanTTL

	lock := fslock.New(trashConfPath)
	lockErr := lock.TryLock()
	if lockErr != nil {
		fmt.Fprintf(os.Stderr, "%s", "try again")
		return
	}
	defer func() {
		lock.Unlock()
		conff, err = json.MarshalIndent(t, "", " ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
		err = ioutil.WriteFile(trashConfPath, conff, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}()

	switch os.Args[1] {
	case "put", "p":
		err = t.put(dir, os.Args[2:]...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", err.Error())
		}
	case "restore", "r":
		err = t.restore(dir, os.Args[2:]...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", err.Error())
		}
	case "ls":
		t.list()
	case "gc":
		err := t.clean()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", err.Error())
		}
	}
}
