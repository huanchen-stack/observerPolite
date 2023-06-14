package main

var SemTLSConn = make(chan struct{}, 30)
var SemGETReq = make(chan struct{}, 30)
