package main

type InstallStartInput struct {
	Hostname string
}

type Status struct {
	Error error
	Done  bool
}
