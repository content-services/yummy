To use this package simply: 

```
import (
	"github.com/content-services/utilities/Rpm"
	"github.com/content-services/utilities/Time"
)
```

and utilize the module methods:

```
 	stop := Time.Elapsed("My print statement")
	print("Woot \n")
	stop()
```

> Woot

> My print statement took 7.5µs

```
	defer Time.ElapsedWithMemory("Total run time")()
	Rpm.Extract("https://download-i2.fedoraproject.org/pub/epel/7/x86_64/")
```

>Total run time took 1.835339084s

>TotalMemoryAllocated = 205 MB
