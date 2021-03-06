// scores.go
//
// Score handling
//
// In here, we have the probing code that learns about scores, reads
// their configuration files, and does the heavy lifting for launching
// them, doing the privilege drop, etc.

package main

import (
	"github.com/kuroneko/configureit"
	"io"
	o "orchestra"
	"os"
	"path"
	"strings"
)

type ScoreInfo struct {
	Name       string
	Executable string

	Interface string

	Config *configureit.Config
}

type ScoreExecution struct {
	Score *ScoreInfo
	Task  *TaskRequest
}

func NewScoreInfo() (si *ScoreInfo) {
	si = new(ScoreInfo)

	config := NewScoreInfoConfig()
	si.updateFromConfig(config)

	return si
}

func NewScoreInfoConfig() (config *configureit.Config) {
	config = configureit.New()

	config.Add("interface", configureit.NewStringOption("env"))
	config.Add("dir", configureit.NewStringOption(""))
	config.Add("path", configureit.NewStringOption("/usr/bin:/bin"))
	config.Add("user", configureit.NewUserOption(""))

	return config
}

func (si *ScoreInfo) updateFromConfig(config *configureit.Config) {
	// set the interface type.
	opt := config.Get("interface")
	sopt, _ := opt.(*configureit.StringOption)
	si.Interface = sopt.Value
}

var (
	Scores map[string]*ScoreInfo
)

func ScoreConfigure(si *ScoreInfo, r io.Reader) {
	o.Info("Score: %s (%s)", (*si).Name, (*si).Executable)
	config := NewScoreInfoConfig()
	err := config.Read(r, 1)
	o.MightFail(err, "Error Parsing Score Configuration for %s", si.Name)
	si.updateFromConfig(config)
}

func LoadScores() {
	scoreDirectory := GetStringOpt("score directory")

	dir, err := os.Open(scoreDirectory)
	o.MightFail(err, "Couldn't open Score directory")
	defer dir.Close()

	Scores = make(map[string]*ScoreInfo)

	files, err := dir.Readdir(-1)
	for i := range files {
		// skip ., .. and other dotfiles.
		if strings.HasPrefix(files[i].Name(), ".") {
			continue
		}
		// emacs backup files.  ignore these.
		if strings.HasSuffix(files[i].Name(), "~") || strings.HasPrefix(files[i].Name(), "#") {
			continue
		}
		// .conf is reserved for score configurations.
		if strings.HasSuffix(files[i].Name(), ".conf") {
			continue
		}
		modetype := files[i].Mode() & os.ModeType
		if modetype != 0 && modetype != os.ModeSymlink {
			continue
		}

		// check for the executionable bit
		if (files[i].Mode().Perm() & 0111) != 0 {
			fullpath := path.Join(scoreDirectory, files[i].Name())
			conffile := fullpath + ".conf"

			si := NewScoreInfo()
			si.Name = files[i].Name()
			si.Executable = fullpath

			conf, err := os.Open(conffile)
			if err == nil {
				ScoreConfigure(si, conf)
				conf.Close()
			} else {
				o.Warn("Couldn't open config file for %s, assuming defaults: %s", files[i].Name, err)
			}
			Scores[files[i].Name()] = si
		}
	}
}
