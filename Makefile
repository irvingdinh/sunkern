.PHONY: install run

install:
	@cd framework && go mod download
	@cd service && go mod download

run:
	@cd service && go run main.go
