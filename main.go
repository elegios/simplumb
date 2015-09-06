package main

import (
	"fmt"
	"bufio"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
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
	if len(targetB) == 0 {
		log.Fatal("Target was empty")
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

	err = os.Chdir(*workingDir)
	if err != nil {
		log.Fatal("Could not change working directory to ", workingDir, ": ", err)
	}

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

func attemptRule(target string, scanner *bufio.Scanner) bool {
	var lastRegexp *regexp.Regexp
	var groups []int
	defer readTillEndOfRule(scanner)
	for scanner.Scan() {
		if scanner.Text() == "" {
			return true
		}
		line := strings.Split(scanner.Text(), " ")

		switch line[0] {
		case "matches":
			if len(line) == 1 {
				log.Fatal("Incorrect syntax on line ", scanner.Text())
			}
			if !match(&target, &lastRegexp, &groups, line[1:]) {
				return false
			}

		case "echo":
			echo(line[1:], target, lastRegexp, groups)

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

func match(target *string, lastRegexp **regexp.Regexp, groups *[]int, args []string) bool {
	re, err := regexp.Compile(strings.Join(args, " "))
	if err != nil {
		log.Fatal("Incorrect regexp: ", err)
	}

	if *lastRegexp == nil {
		hits := re.FindAllStringIndex(*target, -1)
		for _, hit := range hits {
			if *cursor < hit[0] || hit[1] < *cursor {
				continue
			}
			*lastRegexp = re
			*target = (*target)[hit[0]:hit[1]]
			return true
		}
		return false
	}

	*groups = re.FindStringSubmatchIndex(*target)
	*lastRegexp = re
	if (*groups)[0] != 0 || (*groups)[1] != len(*target) {
		return false
	}
	return true
}

func echo(args []string, target string, re *regexp.Regexp, groups []int) {
	fmt.Println(string(re.ExpandString(nil, strings.Join(args, " "), target, groups)))
}
