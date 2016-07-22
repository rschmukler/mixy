package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sync"
)

var regGetAppName = regexp.MustCompile(`app: :(\w+)`)
var regAppDeclaration = regexp.MustCompile(`==> (\w+)`)
var regexHasPath = regexp.MustCompile(`((\w|-)+/)+(\w|-)+\.(\w|-)+`)

type AppResolution struct {
	Name string
	Path string
}

func main() {
	cmd := exec.Command("mix", os.Args[1:]...)

	if !isUmbrella() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		exit(cmd.Run())
	}

	dict := buildAppDictionary()

	out, err := cmd.CombinedOutput()
	var (
		scanner     = bufio.NewScanner(bytes.NewReader(out))
		currentPath = ""
	)

	for scanner.Scan() {
		line := scanner.Bytes()
		if appName, isChange := isAppChange(line); isChange {
			currentPath = dict[appName]
		}
		fmt.Fprintln(os.Stdout, string(prefixPaths(line, currentPath)))
	}
	exit(err)
}

func exit(err error) {
	if err == nil {
		os.Exit(0)
	}
	os.Exit(1)
}

func prefixPaths(line []byte, with string) []byte {
	result := regexHasPath.ReplaceAllFunc(line, func(filePath []byte) []byte {
		result := []byte(path.Join(with, string(filePath)))
		return result
	})
	return result
}

func isUmbrella() bool {
	_, err := os.Stat("apps")
	if os.IsNotExist(err) {
		return false
	} else if err != nil {
		panic(err)
	}
	return true
}

func isAppChange(line []byte) (string, bool) {
	matches := regAppDeclaration.FindSubmatch(line)
	if len(matches) == 0 {
		return "", false
	}

	return string(matches[1]), true
}

func echoAndQuit() {
	buf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(buf))
	os.Exit(0)
}

func buildAppDictionary() map[string]string {
	mixPaths, _ := filepath.Glob("apps/*/mix.exs")
	var (
		recs   = make(chan AppResolution, len(mixPaths))
		result = map[string]string{}
		wg     = sync.WaitGroup{}
	)
	wg.Add(len(mixPaths))
	go func() {
		wg.Wait()
		close(recs)
	}()
	for _, path := range mixPaths {
		getAppName(path, recs, &wg)
	}

	for res := range recs {
		result[res.Name] = res.Path
	}
	return result
}

func getAppName(mixFile string, out chan AppResolution, wg *sync.WaitGroup) {
	defer wg.Done()
	contents, err := ioutil.ReadFile(mixFile)
	if err != nil {
		panic(err)
	}

	name := regGetAppName.FindSubmatch(contents)

	dir, _ := path.Split(mixFile)
	out <- AppResolution{Name: string(name[1]), Path: dir}
}
