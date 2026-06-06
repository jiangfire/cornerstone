//go:build !race

package services

func raceEnabled() bool {
	return false
}
