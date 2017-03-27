########################################################################################

# This Makefile generated by GoMakeGen 0.4.1 using next command:
# gomakegen .

########################################################################################

.PHONY = fmt all clean deps

########################################################################################

all: terrafarm

terrafarm:
	go build terrafarm.go

deps:
	git config --global http.https://gopkg.in.followRedirects true
	git config --global http.https://pkg.re.followRedirects true
	go get -d -v github.com/yosida95/golang-sshkey
	go get -d -v golang.org/x/crypto
	go get -d -v gopkg.in/hlandau/passlib.v1
	go get -d -v pkg.re/essentialkaos/ek.v7

fmt:
	find . -name "*.go" -exec gofmt -s -w {} \;

clean:
	rm -f terrafarm

########################################################################################
