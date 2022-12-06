
# Description
A simple Go based Yum metadata parser with fetching capabilities.

# Installation

```shell  
 go get github.com/content-services/yummy  
```  

# Usage

**To get yum repository metadata from  a URL**
```go   
// Define repository settings and create new repository 
url := "https://download-i2.fedoraproject.org/pub/epel/7/x86_64/"
client := &http.Client { 
     Timeout: time.Second*10 
 }
 
settings := YummySettings{
    Client: client,
    URL:    url,
}

repo, err := NewRepository(settings)

// To get repomd metadata
repomd, statusCode, err := repo.Repomd()

// To get package metadata
packages, statusCode, err := repo.Packages()

// To get repository signature
signature, statusCode, err := repo.Signature()
```  

**To parse packages from a yum repository on disk**

```go  
 xmlFile, err := os.Open("/some/yum/repo/repodata/primary.xml.gz") 
 if err != nil { 
	 log.Fatal(err) 
 } 
 defer xmlFile.Close() 
 result, err := ParseCompressedXMLData(xmlFile)  
```

**To get a GPG Key from a URL**
```go
url := "https://packages.microsoft.com/keys/microsoft.asc"
client := http.Client { Timeout: time.Second*10 }   
gpgKey, statusCode, err := FetchGPGKey(url, client)
```
