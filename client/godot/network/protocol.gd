class_name Protocol
extends RefCounted

enum MessageType {
	REQUEST = 1,
	RESPONSE = 2,
	NOTIFY = 3,
	ERROR = 4
}

enum RouteFlag {
	ROUTE_ID = 0x01,
	ROUTE_STRING = 0x00
}

const DEFAULT_PORT := 3010
const DEFAULT_TIMEOUT := 5000