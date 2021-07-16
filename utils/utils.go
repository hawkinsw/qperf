package utils

import "strconv"

func CreateCompleteNetworkAddress(ipAddress string, portNumber uint64) string {
	return ipAddress + ":" + strconv.FormatUint(portNumber, 10)
}
