/**
 * Dronesmith API
 *
 * Authors
 *  Geoff Gardner <geoff@dronesmith.io>
 *
 * Copyright (C) 2016 Dronesmith Technologies Inc, all rights reserved.
 * Unauthorized copying of any source code or assets within this project, via
 * any medium is strictly prohibited.
 *
 * Proprietary and confidential.
 */
 
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	infile  = flag.String("f", "", "mavlink definition file input")
	outfile = flag.String("o", "", "output file name; default input.go")
)

func main() {

	log.SetFlags(0)
	log.SetPrefix("generator: ")
	flag.Parse()

	fin, err := os.Open(*infile)
	if err != nil {
		usage()
		log.Fatal("Input: ", err)
	}
	defer fin.Close()

	d, err := ParseDialect(fin, baseName(*infile))
	if err != nil {
		log.Fatal("Parse: ", err)
	}

	fout, err := os.Create(findOutFile())
	if err != nil {
		log.Fatal("Output: ", err)
	}
	defer fout.Close()

	if err := d.GenerateGo(fout); err != nil {
		log.Fatal("Generate: ", err)
	}
}

// helper to remove the extension from the base name
func baseName(s string) string {
	return strings.TrimSuffix(filepath.Base(s), filepath.Ext(s))
}

func findOutFile() string {
	if *outfile == "" {
		*outfile = strings.ToLower(baseName(*infile)) + ".go"
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal("Getwd(): ", err)
	}

	return filepath.Join(dir, strings.ToLower(*outfile))
}

func usage() {
	log.Println("Generator - Parse MAVLink XML and create Go file.")
	log.Println("\t-f\tInput File Path")
	log.Println("\t-o\tOutput File Path")
}
