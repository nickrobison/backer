package main

type RPC struct{}

func (r *RPC) SayHello() {
	// logger.Println("Calling RPC server")
	// client, err := rpc.Dial("unix", "/tmp/backer.sock")
	// if err != nil {
	// 	logger.Fatalln(err)
	// }
	// defer client.Close()

	// var reply string
	// err = client.Call("RPC.SayHello", 0, &reply)
	// if err != nil {
	// 	logger.Fatalln(err)
	// }
	// logger.Println("Has response", reply)
}
