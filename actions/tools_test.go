package actions

import (
	"testing"
	"math/rand"
)

func TestApplyPrint(t *testing.T) {

}

func TestTableRows(t *testing.T) {
	for i := 0; i < 10; i++ {
		x := ""
		for i := 0; i < rand.Intn(20); i++ {
			x += "a"
		}

		tableRow(x, "longsdfgsdf", "sfdgsdf", "sdfss", "adsfdfgsdfgsdfgsdfgsdfgsdfgsfg")
	}
}
