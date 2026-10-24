package main

import "flag"

var (
	flagServicesOpen   = flag.Bool("services-open", false, "open the sidebar's Services footer at start, for -shot")
	flagServiceConsole = flag.String("service-console", "", "open this service's console (its id) at start, for -shot")
)

// servicesShot puts the Services footer and a console where the flags ask.
func (u *window) servicesShot() {
	u.sidebar.services.open = *flagServicesOpen
	if *flagServiceConsole != "" {
		u.model.openConsole(*flagServiceConsole)
	}
}
