package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
)

const preamble = `
# Amalgamated hosts file
#
# This is a hosts file generated from various sources.
#
# Date: {{ .date }}
# Number of unique domains: {{ .count }}
#
# Included hosts lists:
{{- range .sources }}
# {{ . }}
{{- end }}
#
# ===============================================================

127.0.0.1 localhost
127.0.0.1 localhost.localdomain
127.0.0.1 local
255.255.255.255 broadcasthost
::1 localhost
::1 ip6-localhost
::1 ip6-loopback
fe80::1%lo0 localhost
ff00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts
0.0.0.0 0.0.0.0

# End preamble

`

var hosts = make(map[string]struct{})
var addrRE = regexp.MustCompile("^[0-9.]*$")

func processHostsFile(contents io.Reader, source string) error {
	foundHosts := 0
	scanner := bufio.NewScanner(contents)
	for scanner.Scan() {
		fieldScanner := bufio.NewScanner(strings.NewReader(scanner.Text()))
		fieldScanner.Split(bufio.ScanWords)
		if !fieldScanner.Scan() || fieldScanner.Text() != "0.0.0.0" {
			// Skip line if it doesn't start with "0.0.0.0"
			continue
		}
		for fieldScanner.Scan() {
			host := fieldScanner.Text()
			if host[0] == '#' {
				break // The rest of the line is a comment
			}
			if addrRE.Match([]byte(host)) {
				fmt.Printf("%s: skipping IP address %s\n", source, host)
				continue
			} else if strings.HasSuffix(host, ".001com") {
				fmt.Printf("%s: skipping invalid host name %s\n", source, host)
				continue
			}
			hosts[host] = struct{}{}
			foundHosts++
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading hosts: %w", err)
	}
	if foundHosts == 0 {
		return fmt.Errorf("did not find any hosts")
	}
	fmt.Printf("%s: found %d hosts\n", source, foundHosts)
	return nil
}

func processSource(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error fetching %s: %w", url, err)
	}
	defer resp.Body.Close()
	err = processHostsFile(resp.Body, url)
	if err != nil {
		return fmt.Errorf("error parsing %s: %w", url, err)
	}
	return nil
}

func writeHosts(sources []string) error {
	hostsFile, err := os.Create("hosts.txt")
	if err != nil {
		return fmt.Errorf("failed to open output hosts.txt: %w", err)
	}
	defer hostsFile.Close()

	var sortedHosts []string

	for host := range hosts {
		sortedHosts = append(sortedHosts, host)
	}

	sort.Slice(sortedHosts, func(i, j int) bool {
		left := strings.Split(sortedHosts[i], ".")
		right := strings.Split(sortedHosts[j], ".")
		leftIndex := len(left)
		rightIndex := len(right)
		for leftIndex > 0 && rightIndex > 0 {
			comp := strings.Compare(left[leftIndex-1], right[rightIndex-1])
			if comp != 0 {
				return comp < 0
			}
			leftIndex--
			rightIndex--
		}
		return leftIndex < 1
	})

	tmpl := template.Must(template.New("").Parse(preamble))
	err = tmpl.Execute(hostsFile, map[string]any{
		"date":    time.Now().UTC().Format(time.RFC1123Z),
		"count":   len(sortedHosts),
		"sources": sources,
	})
	if err != nil {
		return fmt.Errorf("failed to write preamble: %w", err)
	}

	for _, host := range sortedHosts {
		_, err = io.WriteString(hostsFile, fmt.Sprintf("0.0.0.0 %s\n", host))
		if err != nil {
			return fmt.Errorf("failed to write %s to hosts file: %w", host, err)
		}
	}

	return nil
}

func run() error {
	var sources []string
	sourcesFile, err := os.Open("sources.txt")
	if err != nil {
		return fmt.Errorf("error opening sources: %w", err)
	}
	defer sourcesFile.Close()
	scanner := bufio.NewScanner(sourcesFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			err = processSource(line)
			if err != nil {
				return fmt.Errorf("error updating source %s: %w", line, err)
			}
			sources = append(sources, line)
		}
	}
	sort.Strings(sources)
	extras, err := os.Open("extras.txt")
	if err == nil {
		defer extras.Close()
		err = processHostsFile(extras, "extras.txt")
		if err != nil {
			return fmt.Errorf("error reading extras: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error reading extras.txt: %w", err)
	}

	err = addMSEdgeBlockList()
	if err != nil {
		return fmt.Errorf("error reading Microsoft Edge abusive list: %w", err)
	}

	err = writeHosts(sources)
	if err != nil {
		return fmt.Errorf("error writing hosts file: %w", err)
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}
