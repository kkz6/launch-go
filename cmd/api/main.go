package main

func main() {
	application := bootstrap()
	application.registerMiddleware()
	application.registerModules()
	application.run()
}
