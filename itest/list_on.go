//go:build itest
// +build itest

package itest

var testCases = []*testCase{
	{
		name: "onion messages",
		test: OnionMessageTestCase,
	},
	{
		name: "message reply path",
		test: ReplyMessageTestCase,
	},
	{
		name: "forward onion messages",
		test: OnionMsgForwardTestCase,
	},
	{
		name: "decode offer",
		test: DecodeOfferTestCase,
	},
	{
		name: "subscribe onion payload",
		test: SubscribeOnionPayload,
	},
}
