/*
Package goexpose is lightweight json server that can map url paths to various tasks.
Main idea is to have possibility to call shell scripts by http request.
These shell scripts can consume mux vars from url(gorilla mux is used).
You should be very careful how you construct your regular expressions, so
you don't open doors to shell injection.
Goexpose supports authorization system (right now only basic auth is supported).
Goexpose can be run on https so if you combine https with strong password, you should
be not vulnerable.
*/
package goexpose
