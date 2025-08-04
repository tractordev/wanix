**the server should use the right mime type if the file exists, but keep in mind wanix serve doesn't serve the current directory, it only serves files it has embedded in it for starting wanix. maybe it should also serve the current dir? is that what you expected?**



**https://github.com/tractordev/wanix/blob/main/cmd/wanix/serve.go shows what its serving which mostly uses the various embed.FS in the project. we could make it serve these on top of the current directory, but we should probably talk about why. we can come back to it though, let me get through these others.**



**2 is also about something that doesn't exist, unless you've added it. rigth now there is no websocket handler. unless youre using code from https://github.com/tractordev/wanix/blob/main/hack/console/console.go**





**4 is not really a problem, but should probably have an issue anyway. it might be related to #176 but as far as i know it should be ok to exist for now**





**3 should be resolved if you rebase with main**





*after reviewing our code inside the wanixCLEAN/wanix dir (which i put in its own branch under github.com/dwycoff2013/wanix:wanix-CONSOLE):*



**ah! first, generally, you're making it so you can shell out of wanix onto the host. that's pretty cool, but if you want to add that we should create a new issue and hash out design details. or if this was a step towards the console issue i was thinking about (#202), i should say the idea was the opposite. use wanix console ... to connect into a wanix environment in the browser**



**but at least if you rebase it should actually work like normal so you can keep going either way**



**i just pushed a fix for the big error, the webassembly error, just a few hours ago**



**if you rebase from main, wanix serve now also serves the current (or specified directory). turns out this is helpful for working with bundles too, which i started today**

