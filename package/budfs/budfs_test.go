package budfs_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/livebud/bud/internal/testdir"

	"github.com/livebud/bud/package/gomod"
	"github.com/livebud/bud/package/virtual"

	"github.com/livebud/bud/internal/is"
	"github.com/livebud/bud/package/budfs"
	"github.com/livebud/bud/package/log/testlog"
)

func TestGenerateFile(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateFile("a.txt", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte("a")
		return nil
	})
	code, err := fs.ReadFile(bfs, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

func TestGenerateDir(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud", func(fsys budfs.FS, dir *budfs.Dir) error {
		dir.GenerateDir("docs", func(fsys budfs.FS, dir *budfs.Dir) error {
			dir.GenerateFile("a.txt", func(fsys budfs.FS, file *budfs.File) error {
				file.Data = []byte("a")
				return nil
			})
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(bfs, "bud/docs/a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

type tailwind struct {
}

func (t *tailwind) GenerateFile(fs budfs.FS, file *budfs.File) error {
	file.Data = []byte("/* tailwind */")
	return nil
}

type svelte struct {
}

func (t *svelte) GenerateFile(fs budfs.FS, file *budfs.File) error {
	file.Data = []byte("/* svelte */")
	return nil
}

func TestFS(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfsfs := budfs.New(fsys, log)
	bfsfs.FileGenerator("bud/public/tailwind/tailwind.css", &tailwind{})
	bfsfs.FileGenerator("bud/view/index.svelte", &svelte{})

	// .
	des, err := fs.ReadDir(bfsfs, ".")
	is.NoErr(err)
	is.Equal(len(des), 1)

	// bud
	is.Equal(des[0].Name(), "bud")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err := des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Mode(), fs.ModeDir)
	stat, err := fs.Stat(bfsfs, "bud")
	is.NoErr(err)
	is.Equal(stat.Mode(), fs.ModeDir)

	// bud/public
	des, err = fs.ReadDir(bfsfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "public")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "public")
	stat, err = fs.Stat(bfsfs, "bud/public")
	is.NoErr(err)
	is.Equal(stat.Name(), "public")

	// return errors for non-existent files
	_, err = bfsfs.Open("bud\\public")
	is.True(errors.Is(err, fs.ErrNotExist))

	// bud/public/tailwind
	des, err = fs.ReadDir(bfsfs, "bud/public/tailwind")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "tailwind.css")
	is.Equal(des[0].IsDir(), false)

	// read bfserated data
	data, err := fs.ReadFile(bfsfs, "bud/public/index.html")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.True(data == nil)
	data, err = fs.ReadFile(bfsfs, "bud/public/tailwind/tailwind.css")
	is.NoErr(err)
	is.Equal(string(data), "/* tailwind */")
	data, err = fs.ReadFile(bfsfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(data), "/* svelte */")

	// run the TestFS compliance test suite
	is.NoErr(fstest.TestFS(bfsfs, "bud/public/tailwind/tailwind.css", "bud/view/index.svelte"))
}

func view() func(fsys budfs.FS, dir *budfs.Dir) error {
	return func(fsys budfs.FS, dir *budfs.Dir) error {
		dir.GenerateFile("index.svelte", func(fsys budfs.FS, file *budfs.File) error {
			file.Data = []byte(`<h1>index</h1>`)
			return nil
		})
		dir.GenerateFile("about/about.svelte", func(fsys budfs.FS, file *budfs.File) error {
			file.Data = []byte(`<h2>about</h2>`)
			return nil
		})
		return nil
	}
}

func TestViewFS(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", view())

	// bud
	des, err := fs.ReadDir(bfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err := des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "view")

	// bud/view
	stat, err := fs.Stat(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
	is.Equal(stat.IsDir(), true)
	is.Equal(stat.Mode(), fs.ModeDir)

	_, err = bfs.Open("about")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))

	_, err = bfs.Open("bud/view/.")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrInvalid))

	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), "<h1>index</h1>")
	code, err = fs.ReadFile(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), "<h2>about</h2>")

	des, err = fs.ReadDir(bfs, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
	is.Equal(des[0].IsDir(), false)
	is.Equal(des[0].Type(), fs.FileMode(0))
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about.svelte")
	is.Equal(fi.Mode(), fs.FileMode(0))
	stat, err = fs.Stat(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(stat.Name(), "about.svelte")

	is.NoErr(fstest.TestFS(bfs, "bud/view/index.svelte", "bud/view/about/about.svelte"))
}

func TestAll(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", view())

	// .
	file, err := bfs.Open(".")
	is.NoErr(err)
	rbfs, ok := file.(fs.ReadDirFile)
	is.True(ok)
	des, err := rbfs.ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "bud")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err := des[0].Info()
	is.NoErr(err)
	is.Equal(fi.IsDir(), true)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.ModeDir)
	is.Equal(fi.Name(), "bud")
	is.Equal(fi.Size(), int64(0))
	is.Equal(fi.Sys(), nil)
	stat, err := file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), ".")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// Stat .
	stat, err = fs.Stat(bfs, ".")
	is.NoErr(err)
	is.Equal(stat.Name(), ".")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir .
	des, err = fs.ReadDir(bfs, ".")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "bud")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)

	// bud
	file, err = bfs.Open("bud")
	is.NoErr(err)
	rbfs, ok = file.(fs.ReadDirFile)
	is.True(ok)
	des, err = rbfs.ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.IsDir(), true)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.ModeDir)
	is.Equal(fi.Name(), "view")
	is.Equal(fi.Size(), int64(0))
	is.Equal(fi.Sys(), nil)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "bud")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// Stat bud
	stat, err = fs.Stat(bfs, "bud")
	is.NoErr(err)
	is.Equal(stat.Name(), "bud")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir bud
	des, err = fs.ReadDir(bfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)

	// bud/view
	file, err = bfs.Open("bud/view")
	is.NoErr(err)
	rbfs, ok = file.(fs.ReadDirFile)
	is.True(ok)
	des, err = rbfs.ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about")
	is.Equal(fi.IsDir(), true)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.ModeDir)
	is.Equal(fi.Size(), int64(0))
	is.Equal(fi.Sys(), nil)
	is.Equal(des[1].Name(), "index.svelte")
	is.Equal(des[1].IsDir(), false)
	is.Equal(des[1].Type(), fs.FileMode(0))
	fi, err = des[1].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "index.svelte")
	is.Equal(fi.IsDir(), false)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.FileMode(0))
	is.Equal(fi.Size(), int64(14))
	is.Equal(fi.Sys(), nil)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// Stat bud
	stat, err = fs.Stat(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir bud
	des, err = fs.ReadDir(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about")
	is.Equal(fi.IsDir(), true)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.ModeDir)
	is.Equal(fi.Size(), int64(0))
	is.Equal(fi.Sys(), nil)
	is.Equal(des[1].Name(), "index.svelte")
	is.Equal(des[1].IsDir(), false)
	is.Equal(des[1].Type(), fs.FileMode(0))
	fi, err = des[1].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "index.svelte")
	is.Equal(fi.IsDir(), false)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.FileMode(0))
	is.Equal(fi.Size(), int64(14))
	is.Equal(fi.Sys(), nil)

	// bud/view/about
	file, err = bfs.Open("bud/view/about")
	is.NoErr(err)
	rbfs, ok = file.(fs.ReadDirFile)
	is.True(ok)
	des, err = rbfs.ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
	is.Equal(des[0].IsDir(), false)
	is.Equal(des[0].Type(), fs.FileMode(0))
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about.svelte")
	is.Equal(fi.IsDir(), false)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.FileMode(0))
	is.Equal(fi.Size(), int64(14))
	is.Equal(fi.Sys(), nil)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "about")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// Stat bud
	stat, err = fs.Stat(bfs, "bud/view/about")
	is.NoErr(err)
	is.Equal(stat.Name(), "about")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir bud
	des, err = fs.ReadDir(bfs, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
	is.Equal(des[0].IsDir(), false)
	is.Equal(des[0].Type(), fs.FileMode(0))
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about.svelte")
	is.Equal(fi.IsDir(), false)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.FileMode(0))
	is.Equal(fi.Size(), int64(14))
	is.Equal(fi.Sys(), nil)

	// bud/view/index.svelte
	// Open
	file, err = bfs.Open("bud/view/index.svelte")
	is.NoErr(err)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "index.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// Stat
	stat, err = fs.Stat(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(stat.Name(), "index.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// ReadFile
	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)

	// bud/view/about/about.svelte
	// Open
	file, err = bfs.Open("bud/view/about/about.svelte")
	is.NoErr(err)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "about.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// Stat
	stat, err = fs.Stat(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(stat.Name(), "about.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// ReadFile
	code, err = fs.ReadFile(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h2>about</h2>`)

	// Run TestFS
	err = fstest.TestFS(bfs, "bud", "bud/view", "bud/view/index.svelte", "bud/view/about/about.svelte")
	is.NoErr(err)
}

func TestDir(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", func(fsys budfs.FS, dir *budfs.Dir) error {
		dir.GenerateDir("about", func(fsys budfs.FS, dir *budfs.Dir) error {
			dir.GenerateDir("me", func(fsys budfs.FS, dir *budfs.Dir) error {
				return nil
			})
			return nil
		})
		dir.GenerateDir("users/admin", func(fsys budfs.FS, dir *budfs.Dir) error {
			return nil
		})
		return nil
	})
	des, err := fs.ReadDir(bfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	des, err = fs.ReadDir(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[1].Name(), "users")
	is.Equal(des[1].IsDir(), true)
	des, err = fs.ReadDir(bfs, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "me")
	is.Equal(des[0].IsDir(), true)
	des, err = fs.ReadDir(bfs, "bud/view/about/me")
	is.NoErr(err)
	is.Equal(len(des), 0)
	des, err = fs.ReadDir(bfs, "bud/view/users")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "admin")
	is.Equal(des[0].IsDir(), true)
	des, err = fs.ReadDir(bfs, "bud/view/users/admin")
	is.NoErr(err)
	is.Equal(len(des), 0)
}

func TestReadFsys(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{
		"a.txt": &virtual.File{Data: []byte("a")},
	}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	code, err := fs.ReadFile(bfs, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

func TestGenerateFileError(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateFile("bud/main.go", func(fsys budfs.FS, file *budfs.File) error {
		return fs.ErrNotExist
	})
	code, err := fs.ReadFile(bfs, "bud/main.go")
	is.True(err != nil)
	is.Equal(err.Error(), `budfs: open "bud/main.go". mergefs: open "bud/main.go". file does not exist. file does not exist`)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(code, nil)
}

func TestHTTP(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", func(fsys budfs.FS, dir *budfs.Dir) error {
		dir.GenerateFile(dir.Relative(), func(fsys budfs.FS, file *budfs.File) error {
			file.Data = []byte(dir.Target() + "'s data")
			return nil
		})
		return nil
	})
	hfs := http.FS(bfs)

	handler := func(w http.ResponseWriter, r *http.Request) {
		file, err := hfs.Open(r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		stat, err := file.Stat()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Add("Content-Type", "text/javascript")
		http.ServeContent(w, r, r.URL.Path, stat.ModTime(), file)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/bud/view/_index.svelte", nil)
	handler(w, r)

	response := w.Result()
	body, err := io.ReadAll(response.Body)
	is.NoErr(err)
	is.Equal(string(body), `bud/view/_index.svelte's data`)
	is.Equal(response.StatusCode, 200)
}

func rootless(fpath string) string {
	parts := strings.Split(fpath, string(filepath.Separator))
	return path.Join(parts[1:]...)
}

// Test inner file and rootless
func TestTargetPath(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", func(fsys budfs.FS, dir *budfs.Dir) error {
		dir.GenerateFile("about/about.svelte", func(fsys budfs.FS, file *budfs.File) error {
			file.Data = []byte(rootless(file.Target()))
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), "view/about/about.svelte")
}

func TestDynamicDir(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", func(fsys budfs.FS, dir *budfs.Dir) error {
		doms := []string{"about/about.svelte", "index.svelte"}
		for _, dom := range doms {
			dom := dom
			dir.GenerateFile(dom, func(fsys budfs.FS, file *budfs.File) error {
				file.Data = []byte(`<h1>` + dom + `</h1>`)
				return nil
			})
		}
		return nil
	})
	des, err := fs.ReadDir(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[1].Name(), "index.svelte")
	des, err = fs.ReadDir(bfs, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
}

func TestBases(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", func(fsys budfs.FS, dir *budfs.Dir) error {
		return nil
	})
	bfs.GenerateDir("bud/controller", func(fsys budfs.FS, dir *budfs.Dir) error {
		return nil
	})
	stat, err := fs.Stat(bfs, "bud/controller")
	is.NoErr(err)
	is.Equal(stat.Name(), "controller")
	stat, err = fs.Stat(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
}

func TestDirUnevenMerge(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", func(fsys budfs.FS, dir *budfs.Dir) error {
		dir.GenerateDir("public", func(fsys budfs.FS, dir *budfs.Dir) error {
			dir.GenerateFile("favicon.ico", func(fsys budfs.FS, file *budfs.File) error {
				file.Data = []byte("cool_favicon.ico")
				return nil
			})
			return nil
		})
		return nil
	})
	bfs.GenerateDir("bud", func(fsys budfs.FS, dir *budfs.Dir) error {
		dir.GenerateDir("controller", func(fsys budfs.FS, dir *budfs.Dir) error {
			dir.GenerateFile("controller.go", func(fsys budfs.FS, file *budfs.File) error {
				file.Data = []byte("package controller")
				return nil
			})
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(bfs, "bud/view/public/favicon.ico")
	is.NoErr(err)
	is.Equal(string(code), "cool_favicon.ico")
	code, err = fs.ReadFile(bfs, "bud/controller/controller.go")
	is.NoErr(err)
	is.Equal(string(code), "package controller")
}

func TestDirMerge(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", func(fsys budfs.FS, dir *budfs.Dir) error {
		dir.GenerateFile("index.svelte", func(fsys budfs.FS, file *budfs.File) error {
			file.Data = []byte(`<h1>index</h1>`)
			return nil
		})
		dir.GenerateDir("somedir", func(fsys budfs.FS, dir *budfs.Dir) error {
			return nil
		})
		return nil
	})
	bfs.GenerateFile("bud/view/view.go", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte(`package view`)
		return nil
	})
	bfs.GenerateFile("bud/view/plugin.go", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte(`package plugin`)
		return nil
	})
	// bud/view
	des, err := fs.ReadDir(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(len(des), 4)
	is.Equal(des[0].Name(), "index.svelte")
	is.Equal(des[0].IsDir(), false)
	is.Equal(des[1].Name(), "plugin.go")
	is.Equal(des[1].IsDir(), false)
	is.Equal(des[2].Name(), "somedir")
	is.Equal(des[2].IsDir(), true)
	is.Equal(des[3].Name(), "view.go")
	is.Equal(des[3].IsDir(), false)
}

// Add the view
func TestAddGenerator(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/view", view())

	// Add the controller
	bfs.GenerateDir("bud/controller", func(fsys budfs.FS, dir *budfs.Dir) error {
		dir.GenerateFile("controller.go", func(fsys budfs.FS, file *budfs.File) error {
			file.Data = []byte(`package controller`)
			return nil
		})
		return nil
	})

	des, err := fs.ReadDir(bfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "controller")
	is.Equal(des[1].Name(), "view")

	// Read from view
	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)

	// Read from controller
	code, err = fs.ReadFile(bfs, "bud/controller/controller.go")
	is.NoErr(err)
	is.Equal(string(code), `package controller`)
}

type commandGenerator struct {
	Input string
}

func (c *commandGenerator) GenerateFile(fsys budfs.FS, file *budfs.File) error {
	file.Data = []byte(c.Input + c.Input)
	return nil
}

func (c *commandGenerator) GenerateDir(fsys budfs.FS, dir *budfs.Dir) error {
	dir.GenerateFile("index.svelte", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte(c.Input + c.Input)
		return nil
	})
	return nil
}

func TestFileGenerator(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.FileGenerator("bud/command/command.go", &commandGenerator{Input: "a"})
	code, err := fs.ReadFile(bfs, "bud/command/command.go")
	is.NoErr(err)
	is.Equal(string(code), "aa")
}

func TestDirGenerator(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.DirGenerator("bud/view", &commandGenerator{Input: "a"})
	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), "aa")
}

// func TestFileServer(t *testing.T) {
// 	is := is.New(t)
// 	bfs := bfsfs.New()
// 	bfs.FileServer("bud/view", &commandGenerator{Input: "a"})
// 	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
// 	is.NoErr(err)
// 	is.Equal(string(code), "a/bud/view/index.svelte")
// }

func TestDotReadDirEmpty(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateFile("bud/bfserate/main.go", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte("package main")
		return nil
	})
	bfs.GenerateFile("go.mod", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte("module pkg")
		return nil
	})
	des, err := fs.ReadDir(bfs, ".")
	is.NoErr(err)
	is.Equal(len(des), 2)
}

func TestEmbedOpen(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.FileGenerator("bud/view/index.svelte", &budfs.EmbedFile{
		Data: []byte(`<h1>index</h1>`),
	})
	bfs.FileGenerator("bud/view/about/about.svelte", &budfs.EmbedFile{
		Data: []byte(`<h1>about</h1>`),
	})
	bfs.FileGenerator("bud/public/favicon.ico", &budfs.EmbedFile{
		Data: []byte(`favicon.ico`),
	})
	// bud/view/index.svelte
	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)
	stat, err := fs.Stat(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(stat.ModTime(), time.Time{})
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)

	// bud/view/about/about.svelte
	code, err = fs.ReadFile(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>about</h1>`)
	stat, err = fs.Stat(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(stat.ModTime(), time.Time{})
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)

	// bud/public/favicon.ico
	code, err = fs.ReadFile(bfs, "bud/public/favicon.ico")
	is.NoErr(err)
	is.Equal(string(code), `favicon.ico`)
	stat, err = fs.Stat(bfs, "bud/public/favicon.ico")
	is.NoErr(err)
	is.Equal(stat.ModTime(), time.Time{})
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)

	// bud/public
	des, err := fs.ReadDir(bfs, "bud/public")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "favicon.ico")
}

func TestGoModGoMod(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateFile("go.mod", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte("module app.com\nrequire mod.test/module v1.2.4")
		return nil
	})
	stat, err := fs.Stat(bfs, "go.mod/go.mod")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(stat, nil)
	stat, err = fs.Stat(bfs, "go.mod")
	is.NoErr(err)
	is.Equal(stat.Name(), "go.mod")
}

func TestGoModGoModEmbed(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.FileGenerator("go.mod", &budfs.EmbedFile{
		Data: []byte("module app.com\nrequire mod.test/module v1.2.4"),
	})
	stat, err := fs.Stat(bfs, "go.mod/go.mod")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(stat, nil)
	stat, err = fs.Stat(bfs, "go.mod")
	is.NoErr(err)
	is.Equal(stat.Name(), "go.mod")
}

func TestMount(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/generator", func(fsys budfs.FS, dir *budfs.Dir) error {
		return dir.Mount(&virtual.Tree{
			"tailwind/tailwind.go": &virtual.File{Data: []byte("package tailwind")},
			"html/html.go":         &virtual.File{Data: []byte("package html")},
			"service.json":         &virtual.File{Data: []byte(`{"name":"service"}`)},
		})
	})
	err := fstest.TestFS(bfs,
		"bud/generator/tailwind/tailwind.go",
		"bud/generator/html/html.go",
		"bud/generator/service.json",
	)
	is.NoErr(err)
}

// Mounts have priority over generators. It probably should be the other way
// around, but it's not trivial to change so we'll avoid this situation for now.
func TestMountPriority(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateFile("bud/generator/service.json", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte(`{"name":"generator service"}`)
		return nil
	})
	bfs.GenerateDir("bud/generator", func(fsys budfs.FS, dir *budfs.Dir) error {
		return dir.Mount(&virtual.Tree{
			"tailwind/tailwind.go": &virtual.File{Data: []byte("package tailwind")},
			"html/html.go":         &virtual.File{Data: []byte("package html")},
			"service.json":         &virtual.File{Data: []byte(`{"name":"mount service"}`)},
		})
	})
	err := fstest.TestFS(bfs,
		"bud/generator/tailwind/tailwind.go",
		"bud/generator/html/html.go",
		"bud/generator/service.json",
	)
	is.NoErr(err)
	code, err := fs.ReadFile(bfs, "bud/generator/service.json")
	is.NoErr(err)
	is.Equal(string(code), `{"name":"mount service"}`)
}

func TestReadDirNotExists(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateFile("bud/controller/controller.go", func(fsys budfs.FS, file *budfs.File) error {
		return fs.ErrNotExist
	})
	des, err := fs.ReadDir(bfs, "bud/controller")
	is.NoErr(err)
	is.Equal(len(des), 0)
}

func TestServeFile(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.ServeFile("duo/view", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte(file.Target() + `'s data`)
		return nil
	})
	des, err := fs.ReadDir(bfs, "duo/view")
	is.True(errors.Is(err, fs.ErrInvalid))
	is.Equal(len(des), 0)

	// _index.svelte
	file, err := bfs.Open("duo/view/_index.svelte")
	is.NoErr(err)
	code, err := fs.ReadFile(bfs, "duo/view/_index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `duo/view/_index.svelte's data`)
	stat, err := file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "_index.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(29))
	is.Equal(stat.Sys(), nil)

	// about/_about.svelte
	file, err = bfs.Open("duo/view/about/_about.svelte")
	is.NoErr(err)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "_about.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(35))
	is.Equal(stat.Sys(), nil)
	code, err = fs.ReadFile(bfs, "duo/view/about/_about.svelte")
	is.NoErr(err)
	is.Equal(string(code), `duo/view/about/_about.svelte's data`)
}

func TestGenerateDirNotExists(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/public", func(fsys budfs.FS, dir *budfs.Dir) error {
		return fs.ErrNotExist
	})
	stat, err := fs.Stat(bfs, "bud/public")
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(stat, nil)
	des, err := fs.ReadDir(bfs, "bud/public")
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(len(des), 0)
}

// Prioritize generators because they're in memory and quicker to determine if
// they're present in mergefs
func TestGeneratorPriority(t *testing.T) {
	is := is.New(t)
	fsys := virtual.Map{
		"a.txt": &virtual.File{Data: []byte("a")},
	}
	log := testlog.New()
	bfs := budfs.New(fsys, log)
	bfs.GenerateFile("a.txt", func(fsys budfs.FS, file *budfs.File) error {
		file.Data = []byte("b")
		return nil
	})
	code, err := fs.ReadFile(bfs, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "b")
}

func TestGlob(t *testing.T) {
	ctx := context.Background()
	is := is.New(t)
	log := testlog.New()
	dir := t.TempDir()
	td := testdir.New(dir)
	td.Files["controller/controller.go"] = "package controller"
	td.Files["controller/_show.go"] = "package controller"
	td.Files["controller/posts/controller.go"] = "package posts"
	td.Files["controller/posts/.show.go"] = "package posts"
	td.Files["controller/_articles/controller.go"] = "package articles"
	td.Files["controller/.users/controller.go"] = "package users"
	err := td.Write(ctx)
	is.NoErr(err)
	module, err := gomod.Find(dir)
	is.NoErr(err)
	bfs := budfs.New(module, log)
	defer bfs.Close()
	bfs.GenerateDir("bud/controller", func(fsys budfs.FS, dir *budfs.Dir) error {
		results, err := fs.Glob(fsys, "controller/**.go")
		if err != nil {
			return err
		} else if len(results) == 0 {
			return fs.ErrNotExist
		}
		dir.GenerateFile("controller.go", func(fsys budfs.FS, file *budfs.File) error {
			file.Data = []byte(strings.Join(results, " "))
			return nil
		})
		return nil
	})
	des, err := fs.ReadDir(bfs, "bud/controller")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "controller.go")
	code, err := fs.ReadFile(bfs, "bud/controller/controller.go")
	is.NoErr(err)
	is.Equal(string(code), "controller/controller.go controller/posts/controller.go")
}

func TestDefer(t *testing.T) {
	ctx := context.Background()
	is := is.New(t)
	log := testlog.New()
	dir := t.TempDir()
	td := testdir.New(dir)
	err := td.Write(ctx)
	is.NoErr(err)
	module, err := gomod.Find(dir)
	is.NoErr(err)
	bfs := budfs.New(module, log)
	called := 0
	bfs.GenerateFile("a.txt", func(fsys budfs.FS, file *budfs.File) error {
		fsys.Defer(func() error {
			called++
			return nil
		})
		file.Data = []byte("b")
		return nil
	})
	code, err := fs.ReadFile(bfs, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "b")
	bfs.Change("a.txt")
	code, err = fs.ReadFile(bfs, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "b")
	is.Equal(called, 0)
	is.NoErr(bfs.Close())
	is.Equal(called, 2)
	is.NoErr(bfs.Close())
	is.Equal(called, 2)
}

type dirFS struct {
	count map[string]int
	dir   string
}

func (fsys *dirFS) Open(name string) (fs.File, error) {
	if name == "bud" || strings.HasPrefix(name, "bud/") {
		return nil, fs.ErrNotExist
	}
	fsys.count[name]++
	return os.Open(filepath.Join(fsys.dir, name))
}

func TestCacheGenerateFile(t *testing.T) {
	ctx := context.Background()
	is := is.New(t)
	log := testlog.New()
	dir := t.TempDir()
	td := testdir.New(dir)
	td.Files["view/index.svelte"] = "index"
	td.Files["view/about/index.svelte"] = "about"
	err := td.Write(ctx)
	is.NoErr(err)
	count := map[string]int{}
	fsys := &dirFS{count, dir}
	bfs := budfs.New(fsys, log)
	bfs.GenerateFile("bud/internal/app/view/view.go", func(fsys budfs.FS, file *budfs.File) error {
		_, err := fs.Stat(fsys, "view/index.svelte")
		if err != nil {
			return err
		}
		_, err = fs.Stat(fsys, "view/about/index.svelte")
		if err != nil {
			return err
		}
		count["bud/internal/app/view/view.go"]++
		file.Data = []byte("package view")
		return nil
	})
	bfs.GenerateFile("bud/internal/app/web/web.go", func(fsys budfs.FS, file *budfs.File) error {
		_, err := fs.Stat(fsys, "bud/internal/app/view/view.go")
		if err != nil {
			return err
		}
		count["bud/internal/app/web/web.go"]++
		file.Data = []byte("package web")
		return nil
	})

	// Default state
	is.Equal(count["bud/internal/app/web/web.go"], 0, "wrong web generator reads")
	is.Equal(count["bud/internal/app/view/view.go"], 0, "wrong view generator reads")
	is.Equal(count["view/index.svelte"], 0, "wrong index.svelte file reads")
	is.Equal(count["view/about/index.svelte"], 0, "wrong about/index.svelte file reads")
	// First sync
	out := virtual.Map{}
	err = bfs.Sync(out, "bud/internal")
	is.NoErr(err)
	is.Equal(count["bud/internal/app/web/web.go"], 1, "wrong web generator reads")
	is.Equal(count["bud/internal/app/view/view.go"], 1, "wrong view generator reads")
	is.Equal(count["view/index.svelte"], 1, "wrong index.svelte file reads")
	is.Equal(count["view/about/index.svelte"], 1, "wrong about/index.svelte file reads")
	// No change because we're only syncing generators and generators are cached
	err = bfs.Sync(out, "bud/internal")
	is.NoErr(err)
	is.Equal(count["view/index.svelte"], 1, "wrong index.svelte file reads")
	is.Equal(count["view/about/index.svelte"], 1, "wrong about/index.svelte file reads")
	is.Equal(count["bud/internal/app/view/view.go"], 1, "wrong view generator reads")
	is.Equal(count["bud/internal/app/web/web.go"], 1, "wrong web generator reads")
	// Increments real files because we're syncing everything, including the 2
	// files directly. The generators still haven't run since the first run though.
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["view/index.svelte"], 2, "wrong index.svelte file reads")
	is.Equal(count["view/about/index.svelte"], 2, "wrong about/index.svelte file reads")
	is.Equal(count["bud/internal/app/view/view.go"], 1, "wrong view generator reads")
	is.Equal(count["bud/internal/app/web/web.go"], 1, "wrong web generator reads")
	// Generators gets re-run again and incremented, as well as the 2 files
	// directly. However the files are only read once and cached, so they only
	// increment by one, despite being read directly by the generator. Generators
	// are also only run once before cached.
	bfs.Change("view/about/index.svelte")
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["view/index.svelte"], 3, "wrong index.svelte file reads")
	is.Equal(count["view/about/index.svelte"], 3, "wrong about/index.svelte file reads")
	is.Equal(count["bud/internal/app/view/view.go"], 2, "wrong view generator reads")
	is.Equal(count["bud/internal/app/web/web.go"], 2, "wrong web generator reads")
}

func TestCacheGenerateDir(t *testing.T) {
	ctx := context.Background()
	is := is.New(t)
	log := testlog.New()
	dir := t.TempDir()
	td := testdir.New(dir)
	td.Files["node_modules/svelte/svelte.ts"] = "svelte"
	err := td.Write(ctx)
	is.NoErr(err)
	count := map[string]int{}
	fsys := &dirFS{count, dir}
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/internal/node_modules", func(fsys budfs.FS, dir *budfs.Dir) error {
		_, err := fs.Stat(fsys, "node_modules")
		if err != nil {
			return err
		}
		count["bud/internal/node_modules"]++
		dir.GenerateFile("svelte.js", func(fsys budfs.FS, file *budfs.File) error {
			_, err := fs.ReadDir(fsys, "node_modules/svelte")
			if err != nil {
				return err
			}
			count["bud/internal/node_modules/svelte.js"]++
			file.Data = []byte("svelte.js")
			return nil
		})
		return nil
	})
	out := virtual.Map{}
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["node_modules/svelte/svelte.ts"], 1, "wrong svelte.ts generator reads")
	is.Equal(count["bud/internal/node_modules"], 1, "wrong node_modules generator reads")
	is.Equal(count["bud/internal/node_modules/svelte.js"], 1, "wrong svelte.js generator reads")
	// Try again without any changes. Files caching is always reset per sync but
	// the generators are cached across syncs.
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["node_modules/svelte/svelte.ts"], 2, "wrong svelte.ts generator reads")
	is.Equal(count["bud/internal/node_modules"], 1, "wrong node_modules generator reads")
	is.Equal(count["bud/internal/node_modules/svelte.js"], 1, "wrong svelte.js generator reads")
	// Changing the node_modules directory should trigger the dir generator to run
	// but not the svelte generator
	bfs.Change("node_modules")
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["node_modules/svelte/svelte.ts"], 3, "wrong svelte.ts generator reads")
	is.Equal(count["bud/internal/node_modules"], 2, "wrong node_modules generator reads")
	is.Equal(count["bud/internal/node_modules/svelte.js"], 1, "wrong svelte.js generator reads")
	// Changing the node_modules/svelte.ts file should trigger the file generator
	// to run but not the node_module directory generator
	bfs.Change("node_modules/svelte/svelte.ts")
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["node_modules/svelte/svelte.ts"], 4, "wrong svelte.ts generator reads")
	is.Equal(count["bud/internal/node_modules"], 2, "wrong node_modules generator reads")
	is.Equal(count["bud/internal/node_modules/svelte.js"], 2, "wrong svelte.js generator reads")
	// Adding a file will reset the file generator because the file generator
	// reads the svelte directory. The directory generator will not increment.
	is.NoErr(os.WriteFile(filepath.Join(dir, "node_modules/svelte/new.ts"), []byte("new"), 0644))
	bfs.Change("node_modules/svelte/new.ts")
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["node_modules/svelte/svelte.ts"], 5, "wrong svelte.ts generator reads")
	is.Equal(count["node_modules/svelte/new.ts"], 1, "wrong svelte.ts generator reads")
	is.Equal(count["bud/internal/node_modules"], 2, "wrong node_modules generator reads")
	is.Equal(count["bud/internal/node_modules/svelte.js"], 3, "wrong svelte.js generator reads")
	// Deleting a file will reset the file generator because the file generator
	// reads the svelte directory. The directory generator will not increment.
	is.NoErr(os.Remove(filepath.Join(dir, "node_modules/svelte/new.ts")))
	bfs.Change("node_modules/svelte/new.ts")
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["node_modules/svelte/svelte.ts"], 6, "wrong svelte.ts generator reads")
	is.Equal(count["node_modules/svelte/new.ts"], 1, "wrong svelte.ts generator reads")
	is.Equal(count["bud/internal/node_modules"], 2, "wrong node_modules generator reads")
	is.Equal(count["bud/internal/node_modules/svelte.js"], 4, "wrong svelte.js generator reads")
}

func TestCacheServeFile(t *testing.T) {
	ctx := context.Background()
	is := is.New(t)
	log := testlog.New()
	dir := t.TempDir()
	td := testdir.New(dir)
	td.Files["node_modules/svelte.js"] = "svelte"
	td.Files["node_modules/uid.js"] = "uid"
	err := td.Write(ctx)
	is.NoErr(err)
	count := map[string]int{}
	fsys := &dirFS{count, dir}
	bfs := budfs.New(fsys, log)
	bfs.ServeFile("bud/internal/node_modules", func(fsys budfs.FS, file *budfs.File) error {
		count["bud/internal/node_modules"]++
		rel := file.Relative()
		code, err := fs.ReadFile(fsys, path.Join("node_modules", rel))
		if err != nil {
			return err
		}
		file.Data = code
		return nil
	})
	// Base state
	is.Equal(count["node_modules/svelte.js"], 0, "wrong svelte.js generator reads")
	is.Equal(count["node_modules/uid.js"], 0, "wrong uid.js generator reads")
	is.Equal(count["bud/internal/node_modules"], 0, "wrong node_modules generator reads")
	// First time read from the generator for svelte.js
	code, err := fs.ReadFile(bfs, "bud/internal/node_modules/svelte.js")
	is.NoErr(err)
	is.Equal(string(code), "svelte", "wrong svelte.js code")
	is.Equal(count["node_modules/svelte.js"], 1, "wrong svelte.js generator reads")
	is.Equal(count["node_modules/uid.js"], 0, "wrong uid.js generator reads")
	is.Equal(count["bud/internal/node_modules"], 1, "wrong node_modules generator reads")
	// First time read from the generator for uid.js
	code, err = fs.ReadFile(bfs, "bud/internal/node_modules/uid.js")
	is.NoErr(err)
	is.Equal(string(code), "uid", "wrong uid.js code")
	is.Equal(count["node_modules/svelte.js"], 1, "wrong svelte.js generator reads")
	is.Equal(count["node_modules/uid.js"], 1, "wrong uid.js generator reads")
	is.Equal(count["bud/internal/node_modules"], 2, "wrong node_modules generator reads")
	// Second time read from the generator for svelte.js should be cached
	code, err = fs.ReadFile(bfs, "bud/internal/node_modules/svelte.js")
	is.NoErr(err)
	is.Equal(string(code), "svelte", "wrong svelte.js code")
	is.Equal(count["node_modules/svelte.js"], 1, "wrong svelte.js generator reads")
	is.Equal(count["node_modules/uid.js"], 1, "wrong uid.js generator reads")
	is.Equal(count["bud/internal/node_modules"], 2, "wrong node_modules generator reads")
	// Mark "node_modules/svelte.js" which should cause the svelte.js generator to
	// run once again
	bfs.Change("node_modules/svelte.js")
	code, err = fs.ReadFile(bfs, "bud/internal/node_modules/svelte.js")
	is.NoErr(err)
	is.Equal(string(code), "svelte", "wrong svelte.js code")
	is.Equal(count["node_modules/svelte.js"], 2, "wrong svelte.js generator reads")
	is.Equal(count["node_modules/uid.js"], 1, "wrong uid.js generator reads")
	is.Equal(count["bud/internal/node_modules"], 3, "wrong node_modules generator reads")
	// Second time read from the generator for svelte.js should be cached
	code, err = fs.ReadFile(bfs, "bud/internal/node_modules/svelte.js")
	is.NoErr(err)
	is.Equal(string(code), "svelte", "wrong svelte.js code")
	is.Equal(count["node_modules/svelte.js"], 2, "wrong svelte.js generator reads")
	is.Equal(count["node_modules/uid.js"], 1, "wrong uid.js generator reads")
	is.Equal(count["bud/internal/node_modules"], 3, "wrong node_modules generator reads")
}

func TestCacheMount(t *testing.T) {
	ctx := context.Background()
	is := is.New(t)
	log := testlog.New()
	dir := t.TempDir()
	td := testdir.New(dir)
	td.Files["view/a.txt"] = "a"
	td.Files["view/b.txt"] = "b"
	err := td.Write(ctx)
	is.NoErr(err)
	count := map[string]int{}
	fsys := &dirFS{count, dir}
	mountfs := budfs.New(fsys, log)
	mountfs.GenerateFile("bud/generator/a.txt", func(fsys budfs.FS, file *budfs.File) error {
		count["bud/generator/a.txt"]++
		code, err := fs.ReadFile(fsys, "view/a.txt")
		if err != nil {
			return err
		}
		file.Data = code
		return nil
	})
	mountfs.GenerateFile("bud/generator/b.txt", func(fsys budfs.FS, file *budfs.File) error {
		count["bud/generator/b.txt"]++
		code, err := fs.ReadFile(fsys, "view/b.txt")
		if err != nil {
			return err
		}
		file.Data = code
		return nil
	})
	bfs := budfs.New(fsys, log)
	bfs.GenerateDir("bud/generator", func(fsys budfs.FS, dir *budfs.Dir) error {
		count["bud/generator"]++
		subfs, err := fs.Sub(mountfs, "bud/generator")
		if err != nil {
			return err
		}
		return dir.Mount(subfs)
	})
	// Base state
	is.Equal(count["view/a.txt"], 0, "wrong view/a.txt file reads")
	is.Equal(count["view/b.txt"], 0, "wrong view/b.txt file reads")
	is.Equal(count["bud"], 0, "wrong bud generator reads")
	is.Equal(count["bud/generator/a.txt"], 0, "wrong bud/generator/a.txt mount reads")
	is.Equal(count["bud/generator/b.txt"], 0, "wrong bud/generator/b.txt mount reads")
	// Initial sync
	out := virtual.Map{}
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	// Multiple reads due to lack of shared caching across budfs:
	// 1. budfs reads "view/a.txt" with an empty cache
	// 2. mountfs reads "view/a.txt" with a different cache
	is.Equal(count["view/a.txt"], 2, "wrong view/a.txt file reads")
	is.Equal(count["view/b.txt"], 2, "wrong view/b.txt file reads")
	is.Equal(count["bud/generator"], 1, "wrong bud generator reads")
	is.Equal(count["bud/generator/a.txt"], 1, "wrong bud/generator/a.txt mount reads")
	is.Equal(count["bud/generator/b.txt"], 1, "wrong bud/generator/b.txt mount reads")
	// Try again to verify only the files themselves changed were read once
	// because we reset the file cache. The generators in mountfs are still
	// cached and weren't run, leading to no additional reads in view/*.txt.
	out = virtual.Map{}
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["view/a.txt"], 3, "wrong view/a.txt file reads")
	is.Equal(count["view/b.txt"], 3, "wrong view/b.txt file reads")
	is.Equal(count["bud/generator"], 1, "wrong bud generator reads")
	is.Equal(count["bud/generator/a.txt"], 1, "wrong bud/generator/a.txt mount reads")
	is.Equal(count["bud/generator/b.txt"], 1, "wrong bud/generator/b.txt mount reads")
	// Change a file and try again. This time there should be 2 new events, once
	// for the file itelf and once for the mount generator reseting its generator
	// cache.
	bfs.Change("view/a.txt")
	mountfs.Change("view/a.txt")
	out = virtual.Map{}
	err = bfs.Sync(out, ".")
	is.NoErr(err)
	is.Equal(count["view/a.txt"], 5, "wrong view/a.txt file reads")
	is.Equal(count["view/b.txt"], 4, "wrong view/b.txt file reads")
	is.Equal(count["bud/generator"], 1, "wrong bud generator reads")
	is.Equal(count["bud/generator/a.txt"], 2, "wrong bud/generator/a.txt mount reads")
	is.Equal(count["bud/generator/b.txt"], 1, "wrong bud/generator/b.txt mount reads")
}

// func TestRemoteFS(t *testing.T) {
// 	is := is.New(t)
// 	parent := func(t testing.TB, cmd *exec.Cmd) {
// 		ctx := context.Background()
// 		is := is.New(t)
// 		log := testlog.New()
// 		dir := t.TempDir()
// 		td := testdir.New(dir)
// 		err := td.Write(ctx)
// 		is.NoErr(err)
// 		module, err := gomod.Find(dir)
// 		is.NoErr(err)
// 		bfs := budfs.New(module, log)
// 		count := 1
// 		bfs.GenerateDir("bud/generator", func(fsys budfs.FS, dir *budfs.Dir) error {
// 			dir.GenerateFile(dir.Relative(), func(fsys budfs.FS, file *budfs.File) error {
// 				command := remotefs.Command{
// 					Env:    cmd.Env,
// 					Stderr: os.Stderr,
// 					Stdout: os.Stdout,
// 				}
// 				remotefs, err := command.Start(ctx, cmd.Path, cmd.Args[1:]...)
// 				if err != nil {
// 					return err
// 				}
// 				defer remotefs.Close()
// 				data, err := fs.ReadFile(remotefs, dir.Relative())
// 				if err != nil {
// 					return err
// 				}
// 				file.Data = []byte(strings.Repeat(string(data), count))
// 				count++
// 				return nil
// 			})
// 			return nil
// 		})
// 		code, err := fs.ReadFile(bfs, "bud/generator/a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "a")
// 		// Cached
// 		code, err = fs.ReadFile(bfs, "bud/generator/a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "a")
// 		// Read new path (uncached)
// 		code, err = fs.ReadFile(bfs, "bud/generator/b.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "bb")
// 		// Update the file
// 		bfs.Update("bud/generator/a.txt")
// 		// Read again
// 		code, err = fs.ReadFile(bfs, "bud/generator/a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "aaa")
// 	}
// 	child := func(t testing.TB) {
// 		ctx := context.Background()
// 		fsys := virtual.Memory{
// 			"a.txt": &fstest.MapFile{Data: []byte("a")},
// 			"b.txt": &fstest.MapFile{Data: []byte("b")},
// 		}
// 		err := remotefs.ServeFrom(ctx, fsys, "")
// 		is.NoErr(err)
// 	}
// 	testsub.Run(t, parent, child)
// }

// func TestMountRemoteFS(t *testing.T) {
// 	ctx := context.Background()
// 	is := is.New(t)
// 	log := testlog.New()
// 	dir := t.TempDir()
// 	td := testdir.New(dir)
// 	err := td.Write(ctx)
// 	is.NoErr(err)
// 	module, err := gomod.Find(dir)
// 	is.NoErr(err)
// 	parent := func(t testing.TB, cmd *exec.Cmd) {
// 		bfs := budfs.New(module, log)
// 		command := remotefs.Command{
// 			Env:    cmd.Env,
// 			Stderr: os.Stderr,
// 			Stdout: os.Stdout,
// 		}
// 		remotefs, err := command.Start(ctx, cmd.Path, cmd.Args[1:]...)
// 		is.NoErr(err)
// 		defer remotefs.Close()
// 		bfs.Mount("bud/generator", remotefs)
// 		code, err := fs.ReadFile(bfs, "bud/generator/a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "a")
// 		// Cached
// 		code, err = fs.ReadFile(bfs, "bud/generator/a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "a")
// 		// Read new path (uncached)
// 		code, err = fs.ReadFile(bfs, "bud/generator/b.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "bb")
// 		// Update the file
// 		bfs.Update("bud/generator/a.txt")
// 		// Read again
// 		code, err = fs.ReadFile(bfs, "bud/generator/a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "a")
// 	}
// 	child := func(t testing.TB) {
// 		count := 1
// 		bfs := budfs.New(module, log)
// 		bfs.GenerateFile("a.txt", func(fsys budfs.FS, file *budfs.File) error {
// 			file.Data = []byte(strings.Repeat(string("a"), count))
// 			count++
// 			return nil
// 		})
// 		bfs.GenerateFile("b.txt", func(fsys budfs.FS, file *budfs.File) error {
// 			file.Data = []byte(strings.Repeat(string("b"), count))
// 			count++
// 			return nil
// 		})
// 		err := remotefs.ServeFrom(ctx, bfs, "")
// 		is.NoErr(err)
// 	}
// 	testsub.Run(t, parent, child)
// }

// type remoteService struct {
// 	cmd     *exec.Cmd
// 	process *remotefs.Process
// }

// func (s *remoteService) GenerateFile(fsys budfs.FS, file *budfs.File) (err error) {
// 	// This remote service depends on the generators
// 	_, err = fs.Glob(fsys, "generator/*/*.go")
// 	if err != nil {
// 		return err
// 	}
// 	if s.process != nil {
// 		if err := s.process.Close(); err != nil {
// 			return err
// 		}
// 	}
// 	command := remotefs.Command{
// 		Env:    s.cmd.Env,
// 		Stderr: os.Stderr,
// 		Stdout: os.Stdout,
// 	}
// 	s.process, err = command.Start(fsys.Context(), s.cmd.Path, s.cmd.Args[1:]...)
// 	if err != nil {
// 		return err
// 	}
// 	fsys.Defer(func() error {
// 		return s.process.Close()
// 	})
// 	file.Data = []byte(s.process.URL())
// 	return nil
// }

// func TestRemoteService(t *testing.T) {
// 	ctx := context.Background()
// 	is := is.New(t)
// 	log := testlog.New()
// 	dir := t.TempDir()
// 	td := testdir.New(dir)
// 	td.Files["generator/tailwind/tailwind.go"] = "package tailwind"
// 	err := td.Write(ctx)
// 	is.NoErr(err)
// 	module, err := gomod.Find(dir)
// 	is.NoErr(err)
// 	parent := func(t testing.TB, cmd *exec.Cmd) {
// 		bfs := budfs.New(module, log)
// 		defer bfs.Close()
// 		bfs.FileGenerator("bud/service/generator.url", &remoteService{cmd: cmd})
// 		// Return a URL to connect to
// 		url, err := fs.ReadFile(bfs, "bud/service/generator.url")
// 		is.NoErr(err)
// 		// Dial that URL
// 		client, err := remotefs.Dial(ctx, string(url))
// 		is.NoErr(err)
// 		defer client.Close()
// 		// Read the remote file
// 		code, err := fs.ReadFile(client, "a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "a")
// 		// Cached
// 		code, err = fs.ReadFile(client, "a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "a")
// 		// Uncached because it's a new file
// 		code, err = fs.ReadFile(client, "b.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "bb")
// 		// Still cached
// 		url, err = fs.ReadFile(bfs, "bud/service/generator.url")
// 		is.NoErr(err)
// 		code, err = fs.ReadFile(client, "a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "a")
// 		code, err = fs.ReadFile(client, "b.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "bb")
// 		// Update a dependency
// 		bfs.Update("generator/tailwind/tailwind.go")
// 		// Should lead to the generator service being uncached again
// 		url2, err := fs.ReadFile(bfs, "bud/service/generator.url")
// 		is.NoErr(err)
// 		is.True(!bytes.Equal(url, url2))
// 		// Dial the new URL
// 		client2, err := remotefs.Dial(ctx, string(url2))
// 		is.NoErr(err)
// 		defer client2.Close()
// 		// Still cached, even though the remote has been restarted
// 		code, err = fs.ReadFile(client2, "a.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "a")
// 		code, err = fs.ReadFile(client2, "b.txt")
// 		is.NoErr(err)
// 		is.Equal(string(code), "bb")
// 	}
// 	child := func(t testing.TB) {
// 		count := 1
// 		bfs := budfs.New(module, log)
// 		defer bfs.Close()
// 		bfs.GenerateFile("a.txt", func(fsys budfs.FS, file *budfs.File) error {
// 			file.Data = []byte(strings.Repeat(string("a"), count))
// 			count++
// 			return nil
// 		})
// 		bfs.GenerateFile("b.txt", func(fsys budfs.FS, file *budfs.File) error {
// 			file.Data = []byte(strings.Repeat(string("b"), count))
// 			count++
// 			return nil
// 		})
// 		err := remotefs.ServeFrom(ctx, bfs, "")
// 		is.NoErr(err)
// 	}
// 	testsub.Run(t, parent, child)
// }
