test := coverage.out

coverage.out:
	go test -coverprofile=coverage.out

.PHONY: test
test:
	go test -v -coverprofile=coverage.out

.PHONY: view-coverage
view-coverage: coverage.out
	go tool cover -html=coverage.out

