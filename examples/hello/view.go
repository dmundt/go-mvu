package main

import "github.com/dmundt/go-mvu/components"

func helloPage() components.Document {
	return components.Document{
		Title: "Hello World!",
		Body:  components.Heading{Level: 1, Text: "Hello World!"},
	}
}
