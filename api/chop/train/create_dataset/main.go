package main

import (
	"fmt"
	"os"

	"github.com/parquet-go/parquet-go"
)

// QAPair matches the struct in our dataset, adding parquet tags
type QAPair struct {
	Prompt     string `parquet:"prompt"`
	Completion string `parquet:"completion"`
}

func main() {
	pairs := []QAPair{
		{Prompt: "How do you say Hello in Albanian?", Completion: "Përshëndetje"},
		{Prompt: "Translate 'Good morning' to Albanian", Completion: "Mirëmëngjes"},
		{Prompt: "How are you doing?", Completion: "Si jeni?"},
		{Prompt: "Albanian for 'Thank you'", Completion: "Faleminderit"},
		{Prompt: "Translate 'Yes'", Completion: "Po"},
		{Prompt: "Translate 'No'", Completion: "Jo"},
		{Prompt: "How do you say 'Please' in Albanian?", Completion: "Të lutem"},
		{Prompt: "Translate 'Goodbye' to Albanian", Completion: "Mirupafshim"},
		{Prompt: "What is the Albanian word for 'Water'?", Completion: "Ujë"},
		{Prompt: "Translate 'Coffee'", Completion: "Kafe"},
	}

	f, err := os.Create("albanian_qa.parquet")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	writer := parquet.NewGenericWriter[QAPair](f)

	_, err = writer.Write(pairs)
	if err != nil {
		panic(err)
	}

	if err := writer.Close(); err != nil {
		panic(err)
	}

	fmt.Println("Successfully generated albanian_qa.parquet!")
}
