package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	d "github.com/mcmap4/gowebdav"
)

func main() {
	root := flag.String("root", os.Getenv("ROOT"), "WebDAV Endpoint [ENV.ROOT]")
	usr := flag.String("user", os.Getenv("USER"), "User [ENV.USER]")
	pw := flag.String("pw", os.Getenv("PASSWORD"), "Password [ENV.PASSWORD]")
	token := flag.String("token", os.Getenv("TOKEN"), "JWT Token [ENV.TOKEN]")
	netrc := flag.String("netrc-file", filepath.Join(getHome(), ".netrc"), "read login from netrc file")
	method := flag.String("X", "", `Method:
	LS <PATH>
	STAT <PATH>

	MKDIR <PATH>
	MKDIRALL <PATH>

	GET <PATH> [<FILE>]
	PUT <PATH> [<FILE>]

	MV <OLD> <NEW>
	CP <OLD> <NEW>

	DEL <PATH>
	`)
	flag.Parse()

	if *root == "" {
		fail("Set WebDAV ROOT")
	}

	if l := len(flag.Args()); l == 0 || l > 2 {
		fail("Unsupported arguments")
	}

	var c *d.Client

	if *token != "" {
		// create option to ignore insecure certificates
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		httpClient := &http.Client{
			Transport: tr,
		}

		// User supplied token, create Client with Bearer token authorization
		c = d.NewClientJWT(*root, *token, httpClient)

	} else {
		// no token supplied, continue with user/pass-based authentication
		if *pw == "" {
			if u, p := d.ReadConfig(*root, *netrc); u != "" && p != "" {
				usr = &u
				pw = &p
			}
		}

		c = d.NewClient(*root, *usr, *pw)
	}

	cmd := getCmd(*method)

	if e := cmd(c, flag.Arg(0), flag.Arg(1)); e != nil {
		fail(e)
	}
}

func fail(err interface{}) {
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(-1)
}

func getHome() string {
	if u, e := user.Current(); e != nil {
		return u.HomeDir
	}
	return os.Getenv("HOME")
}

func getCmd(method string) func(c *d.Client, p0, p1 string) error {
	switch strings.ToUpper(method) {
	case "LS", "LIST", "PROPFIND":
		return cmdLs

	case "STAT":
		return cmdStat

	case "GET", "PULL", "READ":
		return cmdGet

	case "DELETE", "RM", "DEL":
		return cmdRm

	case "MKCOL", "MKDIR":
		return cmdMkdir

	case "MKCOLALL", "MKDIRALL", "MKDIRP":
		return cmdMkdirAll

	case "RENAME", "MV", "MOVE":
		return cmdMv

	case "COPY", "CP":
		return cmdCp

	case "PUT", "PUSH", "WRITE":
		return cmdPut

	default:
		return func(c *d.Client, p0, p1 string) (err error) {
			return errors.New("Unsupported method: " + method)
		}
	}
}

func cmdLs(c *d.Client, p0, _ string) (err error) {
	files, err := c.ReadDir(p0)
	if err == nil {
		fmt.Println(fmt.Sprintf("ReadDir: '%s' entries: %d ", p0, len(files)))
		for _, f := range files {
			fmt.Println(f)
		}
	}
	return
}

func cmdStat(c *d.Client, p0, _ string) (err error) {
	file, err := c.Stat(p0)
	if err == nil {
		fmt.Println(file)
	}
	return
}

func cmdGet(c *d.Client, p0, p1 string) (err error) {
	bytes, err := c.Read(p0)
	if err == nil {
		if p1 == "" {
			p1 = filepath.Join(".", p0)
		}
		err = writeFile(p1, bytes, 0644)
		if err == nil {
			fmt.Println(fmt.Sprintf("Written %d bytes to: %s", len(bytes), p1))
		}
	}
	return
}

func cmdRm(c *d.Client, p0, _ string) (err error) {
	if err = c.Remove(p0); err == nil {
		fmt.Println("Remove: " + p0)
	}
	return
}

func cmdMkdir(c *d.Client, p0, _ string) (err error) {
	if err = c.Mkdir(p0, 0755); err == nil {
		fmt.Println("Mkdir: " + p0)
	}
	return
}

func cmdMkdirAll(c *d.Client, p0, _ string) (err error) {
	if err = c.MkdirAll(p0, 0755); err == nil {
		fmt.Println("MkdirAll: " + p0)
	}
	return
}

func cmdMv(c *d.Client, p0, p1 string) (err error) {
	if err = c.Rename(p0, p1, true); err == nil {
		fmt.Println("Rename: " + p0 + " -> " + p1)
	}
	return
}

func cmdCp(c *d.Client, p0, p1 string) (err error) {
	if err = c.Copy(p0, p1, true); err == nil {
		fmt.Println("Copy: " + p0 + " -> " + p1)
	}
	return
}

func cmdPut(c *d.Client, p0, p1 string) (err error) {
	if p1 == "" {
		p1 = filepath.Join(".", p0)
	}
	stream, err := getStream(p1)
	if err != nil {
		return
	}
	defer stream.Close()

	if err = c.WriteStream(p0, stream, 0644); err == nil {
		fmt.Println("Put: " + p1 + " -> " + p0)
	}
	return
}

func writeFile(path string, bytes []byte, mode os.FileMode) error {
	parent := filepath.Dir(path)
	if _, e := os.Stat(parent); os.IsNotExist(e) {
		if e := os.MkdirAll(parent, os.ModePerm); e != nil {
			return e
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(bytes)
	return err
}

func getStream(pathOrString string) (io.ReadCloser, error) {

	fi, err := os.Stat(pathOrString)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return nil, &os.PathError{
			Op:   "Open",
			Path: pathOrString,
			Err:  errors.New("Path: '" + pathOrString + "' is a directory"),
		}
	}

	f, err := os.Open(pathOrString)
	if err == nil {
		return f, nil
	}

	return nil, &os.PathError{
		Op:   "Open",
		Path: pathOrString,
		Err:  err,
	}
}
