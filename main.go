package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
)

var (
	cmdRe  = regexp.MustCompile(`(\w+)(?: (.+))?`)
	repRe  = regexp.MustCompile(`\$(\d+|file|dir|path|target|fulltarget|\$)`)
	homeRe = regexp.MustCompile(`^~([^/]*)(?:/|$)`)
)

var (
	plumbPath  = flag.String("rule-file", "~/.kak-plumb", "The path to the rules file")
	workingDir = flag.String("working-dir", ".", "The path to the directory in which the source is located")
	cursor     = flag.Int("cursor", 0, "The position within the string at which the cursor was, if applicable")
)

func init() {
	flag.StringVar(plumbPath, "r", "~/.kak-plumb", "The path to the rules file")
	flag.StringVar(workingDir, "d", ".", "The path to the directory in which the source is located")
	flag.IntVar(cursor, "c", 0, "The position within the string at which the cursor was, if applicable")
}

func main() {
	flag.Parse()
	targetB, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal("Could not read target: ", err)
	}
	target := string(targetB)

	// convert cursor to a byte index
	for index := range target {
		if *cursor <= 0 {
			*cursor = index
			break
		}
		*cursor -= 1
	}

	*workingDir = fixhome(*workingDir)
	err = os.Chdir(*workingDir)
	if err != nil {
		log.Fatal("Could not change working directory to ", workingDir, ": ", err)
	}

	*plumbPath = fixhome(*plumbPath)
	file, err := os.Open(*plumbPath)
	if err != nil {
		log.Fatal("Could not open the rule file: ", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for {
		if attemptRule(target, scanner) {
			break
		}
	}
}

type ruleState struct {
	fullTarget, target string
	groups             []int
	file, dir, path    string
	firstMatchDone     bool
}

func attemptRule(target string, scanner *bufio.Scanner) bool {
	state := ruleState{fullTarget: target, target: target}

	defer readTillEndOfRule(scanner)
	for scanner.Scan() {
		cmd := cmdRe.FindStringSubmatch(scanner.Text())
		if cmd == nil {
			return true
		}

		switch cmd[1] {
		case "matches":
			if !match(&state, cmd[2]) {
				return false
			}

		case "echo":
			echo(state, cmd[2])

		case "isfile":
			if !checkpath(&state, cmd[2], "file") {
				return false
			}

		case "isdir":
			if !checkpath(&state, cmd[2], "dir") {
				return false
			}

		case "isexist":
			if !checkpath(&state, cmd[2], "exist") {
				return false
			}

		case "isnotexist":
			if !checkpath(&state, cmd[2], "notexist") {
				return false
			}

		default:
			log.Fatal("Could not parse line: ", scanner.Text())
		}
	}

	return true
}

func readTillEndOfRule(scanner *bufio.Scanner) {
	for scanner.Text() != "" && scanner.Scan() {
	}
}

func match(r *ruleState, arg string) bool {
	if arg == "" {
		log.Fatal("matches needs a regexp as an argument, got none")
	}
	re, err := regexp.Compile(arg)
	if err != nil {
		log.Fatal("Incorrect regexp: ", err)
	}

	if !r.firstMatchDone {
		hits := re.FindAllStringSubmatchIndex(r.target, -1)
		for _, hit := range hits {
			if *cursor < hit[0] || hit[1] < *cursor {
				continue
			}
			r.firstMatchDone = true
			r.target = r.target[hit[0]:hit[1]]
			shift := hit[0]
			for i := range hit {
				hit[i] -= shift
			}
			r.groups = hit
			return true
		}
		return false
	}

	r.groups = re.FindStringSubmatchIndex(r.target)
	if r.groups == nil || r.groups[0] != 0 || r.groups[1] != len(r.target) {
		return false
	}
	return true
}

func echo(r ruleState, arg string) {
	fmt.Println(format(arg, r))
}

func fixhome(path string) string {
	match := homeRe.FindStringSubmatch(path)
	if match == nil {
		return path
	}

	var usr *user.User
	var err error
	if match[1] == "" {
		usr, err = user.Current()
	} else {
		usr, err = user.Lookup(match[1])
	}
	if err != nil {
		return path
	}
	return filepath.Join(usr.HomeDir, path[len(match[0]):])
}

func checkpath(r *ruleState, arg string, kind string) bool {
	path, err := filepath.Abs(fixhome(format(arg, *r)))
	if err != nil {
		log.Println("Could not determine the path of ", arg, ": ", err)
		return false
	}
	r.path = path
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return kind == "notexist"
	}
	if err != nil {
		log.Println("Could not examine path ", path, ": ", err)
		return false
	}
	if kind == "notexist" {
		return false
	}
	if kind == "exist" {
		return true
	}
	if kind == "dir" {
		r.dir = path
	} else {
		r.file = path
	}
	return (kind == "dir") == fi.IsDir()
}

func format(template string, r ruleState) string {
	return repRe.ReplaceAllStringFunc(template, func(pattern string) string {
		pattern = pattern[1:]
		switch pattern {
		case "$":
			return "$"
		case "file":
			return r.file
		case "dir":
			return r.dir
		case "path":
			return r.path
		case "target":
			return r.target
		case "fulltarget":
			return r.fullTarget
		default:
			gn, err := strconv.Atoi(pattern)
			if err != nil {
				log.Fatal("Implementation error, couldn't parse ", pattern, " as an int: ", err)
			}
			if gn*2 >= len(r.groups) || r.groups[gn*2] == -1 {
				return ""
			}
			return r.target[r.groups[gn*2]:r.groups[gn*2+1]]
		}
	})
}
