package main

func shouldBootstrapServices(stateDBExisted bool, serviceCount int) bool {
	return !stateDBExisted && serviceCount == 0
}
