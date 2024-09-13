package cmd

import "testing"

func TestLog(t *testing.T) {
	c := CMD{
		PostgresUrl: "postgres://testing:testing@postgres:5432/testing?sslmode=disable",
	}
	t.Log(c.AsLog())
}
