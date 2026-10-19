// Exact forward41 owner guard bodies; native39 and current11 stay unchanged.
package store

import "voice/backend/pkg/authoritysource"

var effectiveMemberAuthorityFunctions = []authoritysource.CatalogFunction{
	{Name: "space_effective_member_head_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "fe84ecc060f27aeff57a711df113f6477cba90cd297ba49ac38699ab9c3b7e13"},
	{Name: "space_effective_member_liveness_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "f892f9d0c2b9b001788c07c52e5c502ad0b5aca05dbc2c83c7c451cb32d982d5"},
	{Name: "space_effective_membership_terminal_cleanup", Arguments: 0, ReturnType: "trigger", BodySHA256: "ea2a9b0dd7826274837561c24fe142fa140a0b690e419e45ec2911aa96f4ee0b"},
}
