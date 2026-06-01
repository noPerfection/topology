module github.com/noPerfection/runtime/config

go 1.19

require github.com/noPerfection/protocol/message v0.0.0

require github.com/noPerfection/datatype v0.0.0 // indirect

replace (
	github.com/noPerfection/datatype => ../../datatype
	github.com/noPerfection/protocol/message => ../../protocol/message
)
