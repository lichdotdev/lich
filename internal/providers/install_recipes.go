package providers

// recipes is each provider's unattended install, by OS and provider id.
var recipes = map[string]map[string]recipe{}

// installGaps is why a provider has no recipe on an OS, by OS and provider
// id; docs/ceilings.md and the README's provider table say the same.
var installGaps = map[string]map[string]string{}
