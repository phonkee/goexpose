package goexpose
import "strings"

/*
Returns if method is allowed

if avail methods is blank it also returns true
 */
func MethodAllowed(method string, avail []string) bool {
	if len(avail) == 0 {
		return true
	}
	for _, am := range avail {
		if strings.ToUpper(method) == strings.ToUpper(am){
			return true
		}
	}
	return false
}
