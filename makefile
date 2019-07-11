GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

BINARYNAME=logstat
COVERAGENAME=coverage.out

all: clean test build

build:
	$(GOBUILD) -o $(BINARYNAME)

test:
	$(GOTEST) ./...

race:
	$(GOTEST) -race -count=1000 ./...

coverage: clean
	$(GOTEST) -coverprofile=$(COVERAGENAME) ./... && go tool cover -html=$(COVERAGENAME)

clean:
	$(GOLEAN)
	rm -f $(BINARYNAME)
	rm -f $(COVERAGENAME)

run:
	$(GOBUILD) -o $(BINARYNAME)
	./$(BINARYNAME)