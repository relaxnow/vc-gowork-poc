#!/bin/sh
# Packager, package thyself!
go run cmd/packager/main.go ../vc-gowork-poc
veracode static scan vc-gowork-poc.zip
rm vc-gowork-poc.zip