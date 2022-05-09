To use this package simply: 

```
import (
	"github.com/content-services/utilities/Rpm"
	"github.com/content-services/utilities/Time"
)
```

and utilize the inherited function/types

```
 	stop := Time.Elapsed("My print statement")
	print("Woot")
	stop() // >> WootMy print statement took 7.875µs
	defer Time.ElapsedWithMemory("Total run time")()
	Rpm.Extract("https://download-i2.fedoraproject.org/pub/epel/7/x86_64/", false)
	// Total run time took 1.835339084s
	// TotalMemoryAllocated = 205 MB
```

Extract accepts an RPM Url and a debug boolean value, set to false for production.