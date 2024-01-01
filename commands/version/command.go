package version

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"runtime/debug"

	"github.com/dihedron/dedup/commands/base"
)

// Version is the command that prints information about the application
// or plugin to the console; it support both compact and verbose mode.
type Version struct {
	base.Command
	// Verbose prints extensive information about this application or plugin.
	Verbose bool `short:"v" long:"verbose" description:"Print extensive information about this application."`
}

type ShortInfo struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Copyright   string `json:"copyright,omitempty"`
	Version     string `json:"version,omitempty"`
}

type DetailedInfo struct {
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Copyright   string       `json:"copyright,omitempty"`
	Version     VersionInfo  `json:"version,omitempty"`
	Compiler    CompilerInfo `json:"compiler,omitempty"`
	Build       BuildInfo    `json:"build,omitempty"`
	Git         GitInfo      `json:"git,omitempty"`
}

type VersionInfo struct {
	Major string `json:"major,omitempty"`
	Minor string `json:"minor,omitempty"`
	Patch string `json:"patch,omitempty"`
}

func (v *VersionInfo) String() string {
	return fmt.Sprintf("%s.%s.%s", v.Major, v.Minor, v.Patch)
}

type BuildInfo struct {
	Time string `json:"time,omitempty"`
	Date string `json:"date,omitempty"`
}

type CompilerInfo struct {
	Version      string `json:"version,omitempty"`
	OS           string `json:"os,omitempty"`
	Architecture string `json:"arch,omitempty"`
}

type GitInfo struct {
	Tag      string `json:"tag,omitempty"`
	Commit   string `json:"commit,omitempty"`
	Time     string `json:"time,omitempty"`
	Modified string `json:"modified,omitempty"`
}

// Execute is the real implementation of the Version command.
func (cmd *Version) Execute(args []string) error {
	cmd.Init()
	slog.Debug("running version command", "name", Name)
	bi, _ := debug.ReadBuildInfo()
	for _, setting := range bi.Settings {
		slog.Debug("debug info setting", "key", setting.Key, "value", setting.Value)
		switch setting.Key {
		case "GOOS":
			GoOS = setting.Value
		case "GOARCH":
			GoArch = setting.Value
		case "vcs.revision":
			GitCommit = setting.Value
		case "vcs.time":
			GitTime = setting.Value
		case "vcs.modified":
			GitModified = setting.Value
		}
	}
	GoVersion = bi.GoVersion

	if cmd.AutomationFriendly {
		var info interface{}
		if !cmd.Verbose {
			// short
			info = &ShortInfo{
				Name:        Name,
				Description: Description,
				Copyright:   Copyright,
				Version:     fmt.Sprintf("v%s.%s.%s", VersionMajor, VersionMinor, VersionPatch),
			}
		} else {
			// verbose
			info = &DetailedInfo{
				Name:        Name,
				Description: Description,
				Copyright:   Copyright,
				Version: VersionInfo{
					Major: VersionMajor,
					Minor: VersionMinor,
					Patch: VersionPatch,
				},
				Build: BuildInfo{
					Time: BuildTime,
				},
				Compiler: CompilerInfo{
					Version:      GoVersion,
					OS:           GoOS,
					Architecture: GoArch,
				},
				Git: GitInfo{
					Tag:      GitTag,
					Time:     GitTime,
					Commit:   GitCommit,
					Modified: GitModified,
				},
			}
		}
		data, err := json.Marshal(info)
		if err != nil {
			slog.Error("error marshalling plugin info to JSON", "error", err)
			return err
		}
		slog.Debug("marshalling data to JSON", "data", string(data))
		fmt.Println(string(data))
	} else {
		if !cmd.Verbose {
			fmt.Printf("\n  %s %s - %s - %s\n\n", path.Base(os.Args[0]), GitTag, Copyright, Description)
		} else {
			if GitTag != "" {
				fmt.Printf("\n  %s %s - %s - %s\n\n", path.Base(os.Args[0]), GitTag, Copyright, Description)
			} else {
				fmt.Printf("\n  %s - %s - %s\n\n", path.Base(os.Args[0]), Copyright, Description)
			}
			fmt.Printf("  - Name             : %s\n", Name)
			fmt.Printf("  - Description      : %s\n", Description)
			fmt.Printf("  - Copyright        : %s\n", Copyright)
			fmt.Printf("  - Major Version    : %s\n", VersionMajor)
			fmt.Printf("  - Minor Version    : %s\n", VersionMinor)
			fmt.Printf("  - Patch Version    : %s\n", VersionPatch)
			fmt.Printf("  - Build Time       : %s\n", BuildTime)
			fmt.Printf("  - Compiler         : %s\n", GoVersion)
			fmt.Printf("  - Operating System : %s\n", GoOS)
			fmt.Printf("  - Architecture     : %s\n", GoArch)
			fmt.Printf("  - Git Tag          : %s\n", GitTag)
			fmt.Printf("  - Git Time         : %s\n", GitTime)
			fmt.Printf("  - Git Commit       : %s\n", GitCommit)
			fmt.Printf("  - Git Modified     : %s\n", GitModified)
		}
	}
	slog.Debug("command done")
	return nil
}

// NOTE: these variables are populated at compile time by using the -ldflags
// linker flag:
//   $> go build -ldflags "-X github.com/dihedron/dedup/commands/version.GitHash=$(hash)"
// in order to get the package path to the GitHash variable to use in the
// linker flag, use the nm utility and look for the variable in the built
// application symbols, then use its path in the linker flag:
//   $> nm ./dedup | grep GitHash
//   00000000015db9c0 b github.com/dihedron/dedup/commands/version.GitHash

var (
	// Name is the name of the application or plugin.
	Name string
	// Description is a one-liner description of the application or plugin.
	Description string
	// Copyright is the copyright clause of the application or plugin.
	Copyright string
	// License is the license under which the code is released.
	License string
	// LicenseURL is the URL at which the license is available.
	LicenseURL string
	// BuildTime is the time at which the application was built.
	BuildTime string
	// GitTag is the current Git tag (e.g. "1.0.3").
	GitTag string
	// GitCommit is the commit of this version of the application.
	GitCommit string
	// GitTime is the modification time associated with the Git commit.
	GitTime string
	// GitModified reports whether the repository had outstanding local changes at time of build.
	GitModified string
	// GoVersion is the version of the Go compiler used in the build process.
	GoVersion string
	// GoOS is the operating system used to build this application; it may differ
	// from that of the compiler in case of cross-compilation (GOOS).
	GoOS string
	// GoOS is the architecture used during the build of this application; it
	// may differ from that of the compiler in case of cross-compilation (GOARCH).
	GoArch string
	// VersionMajor is the major version of the application.
	VersionMajor = "0"
	// VersionMinor is the minor version of the application.
	VersionMinor = "0"
	// VersionPatch is the patch or revision level of the application.
	VersionPatch = "0"
)

func init() {
	if Name == "" {
		Name = path.Base(os.Args[0])
	}
}
