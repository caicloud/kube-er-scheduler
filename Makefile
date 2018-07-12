.PHONY: build clean test-update test-update-er7

build:
	# build
	go build -o kube-er-scheduler *.go

clean:
	# delete build file
	rm -rf ./kube-er-scheduler

test-update:
	# test TestUpdateNode
	go test -v -run TestUpdateNode$

test-update-er7:
	# test TestUpdateNodeER7
	go test -v -run TestUpdateNodeER7