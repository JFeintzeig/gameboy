app:
	go build cmd/app/app.go

test: app
	./scripts/cpu_instrs_test.sh
