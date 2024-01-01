NAME := dedup
DESCRIPTION := Detect duplicate files on disk.
COPYRIGHT := 2024 © Andrea Funtò
LICENSE := MIT
LICENSE_URL := https://opensource.org/license/mit/
VERSION_MAJOR := 0
VERSION_MINOR := 0
VERSION_PATCH := 1

CGO_ENABLED = 1

#
# Linux x86-64 build settings
#
linux/amd64: GOAMD64 = v3

#
# Windows x86-64 build settings
#
windows/amd64: GOAMD64 = v3


.PHONY: default
default: linux/amd64 ;

%:
	@go mod tidy
	@go generate ./...    
	@for platform in "$(platforms)"; do \
		if test "$(@)" = "$$platform"; then \
			echo "Building target $(@)..."; \
			mkdir -p dist/$(@); \
			GOOS=$(shell echo $(@) | cut -d "/" -f 1) GOARCH=$(shell echo $(@) | cut -d "/" -f 2) GOAMD64=$(GOAMD64) CGO_ENABLED=$(CGO_ENABLED) go build -v -ldflags="-X '$(package).Name=$(NAME)' -X '$(package).Description=$(DESCRIPTION)' -X '$(package).Copyright=$(COPYRIGHT)' -X '$(package).License=$(LICENSE)' -X '$(package).LicenseURL=$(LICENSE_URL)' -X '$(package).BuildTime=$(now)' -X '$(package).VersionMajor=$(VERSION_MAJOR)' -X '$(package).VersionMinor=$(VERSION_MINOR)' -X '$(package).VersionPatch=$(VERSION_PATCH)'" -o dist/$(@)/ .;\
			echo ...done!; \
		fi; \
	done

.PHONY: run
run:
	@dist/linux/amd64/dedup index --directory=./staging/offline/ --database=./test/my.db --log-level=error

.PHONY: clean
clean:
	@rm -rf dist

platforms="$$(go tool dist list)"
	
module := $$(grep "module .*" go.mod | sed 's/module //gi')
package := $(module)/commands/version
now := $$(date --rfc-3339=seconds)
