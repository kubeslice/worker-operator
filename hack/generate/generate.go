package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"gopkg.in/yaml.v2"
)

var (
	templateFilePath = "hack/templates/schema.tmpl"
	outputFilePath   = "events/events_generated.go"
)

func main() {
	fmt.Println("generating event schema code from schema file")
	if len(os.Args) < 2 {
		handleError(fmt.Errorf("must pass schema file path"))
	}

	events, err := parseEvent(os.Args[1])
	handleError(err)

	t, err := template.ParseFiles(templateFilePath)
	handleError(err)

	f, err := os.Create(outputFilePath)
	handleError(err)
	t.Execute(f, events)
}

func handleError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func parseEvent(filepath string) ([]events.EventSchema, error) {
	var eventSchema struct {
		Events []events.EventSchema
	}
	event, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(event, &eventSchema)
	if err != nil {
		return nil, err
	}
	return eventSchema.Events, nil
}
