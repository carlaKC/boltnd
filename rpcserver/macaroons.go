package rpcserver

import "gopkg.in/macaroon-bakery.v2/bakery"

// RPCServerPermissions is the set of macaroon permissions needed for each
// api.
var RPCServerPermissions = map[string][]bakery.Op{
	"/offersrpc.Offers/SendOnionMessage": {{
		Entity: "peers",
		Action: "write",
	}},
	"/offersrpc.Offers/DecodeOffer": {{
		Entity: "offchain",
		Action: "read",
	}},
	"/offersrpc.Offers/SubscribeOnionPayload": {{
		Entity: "offchain",
		Action: "read",
	}},
}
