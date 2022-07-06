# Description 

A simple Go based Yum metadata parser with fetching capabilities.

Currently, this project currently only parses package information. 

# Installation

```shell
 go get github.com/content-services/yummy
```

# Usage

```go
import (
	"github.com/content-services/yummy/yum"
)
```
and fetch and parse the packages from a yum repository:

```go
	yum.ExtractPackageData(http.Client{}, "https://download-i2.fedoraproject.org/pub/epel/7/x86_64/")
```

to parse the packages from a yum repository on disk:

```go
	xmlFile, err := os.Open("/some/yum/repo/repodata/primary.xml.gz")
	if err != nil {
		log.Fatal(err)
	}
	defer xmlFile.Close()
	result, err := ParseCompressedXMLData(xmlFile)
```

