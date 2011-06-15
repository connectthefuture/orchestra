package main

import (
	"path"
	"os"
	"bufio"
	o "orchestra"
	"strings"
	"github.com/kless/goconfig/config"
)


/* we actually use this as a cheap set, rather than a map */
var	authorisedHosts		map[string]bool

var	conf	*config.Config = nil

func init() {
	resetConfig()
}

func resetConfig() {
	conf = config.NewDefault()
	conf.AddSection("Conductor")
	conf.AddOption("Conductor", "playerfile", "players")
	conf.AddOption("Conductor", "bindaddress", "")
	conf.AddOption("Conductor", "private_key", "conductor_key.pem")
	conf.AddOption("Conductor", "certificate", "conductor_crt.pem")
}


func pathFor(shortname string) (fullpath string) {
	return path.Clean(path.Join(*ConfigDirectory, shortname))
}

func ConfigLoad() {
	resetConfig()

	pfh, err := os.Open(pathFor("players"))
	o.MightFail("Couldn't open \"players\": %s", err)

	pbr := bufio.NewReader(pfh)

	newAuthorisedHosts := make(map[string]bool)
	for err = nil; err == nil; {
		var lb		[]byte
		var prefix	bool

		lb, prefix, err = pbr.ReadLine()

		if nil == lb {
			break;
		}
		if prefix {
			o.Fail("ConfigLoad: Short Read (prefix only)!")
		}
		
		line := strings.TrimSpace(string(lb))
		if line[0] == '#' {
			continue;
		}
		newAuthorisedHosts[line] = true
	}
	authorisedHosts = newAuthorisedHosts

}


func HostAuthorised(hostname string) (r bool) {
	/* if we haven't loaded the configuration, nobody is authorised */
	if authorisedHosts == nil {
		return false
	}
	_, exists := authorisedHosts[hostname]

	return exists	
}