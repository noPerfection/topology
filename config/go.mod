module github.com/noPerfection/topology/config

go 1.22

require (
	github.com/ahmetson/mushroom v0.0.0
	github.com/noPerfection/datatype v0.0.0
	github.com/noPerfection/protocol/message v0.0.0
)

replace (
	github.com/ahmetson/mushroom => ../../../ahmetson/mushroom
	github.com/noPerfection/datatype => ../../datatype
	github.com/noPerfection/protocol/message => ../../protocol/message
)
