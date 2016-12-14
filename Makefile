########################################################################################

.PHONY = terrafarm clean fmt deps

########################################################################################

all: terrafarm

deps:
	go get -v pkg.re/essentialkaos/ek.v5
	go get -v pkg.re/essentialkaos/go-linenoise.v3
	go get -v gopkg.in/hlandau/passlib.v1
	go get -v github.com/yosida95/golang-sshkey

fmt:
	find . -name "*.go" -exec gofmt -s -w {} \;

terrafarm:
	go build terrafarm.go

clean:
	rm -f terrafarm
