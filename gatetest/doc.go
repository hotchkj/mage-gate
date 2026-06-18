// Package gatetest provides test doubles for the gate library: fake runners, ToolResolver fakes,
// file operations, response builders, and command key constants for tests that extend gate steps.
// Its resolver helpers fake the ToolResolver boundary for gate consumers; they do not construct
// cmdrunner's concrete production resolver. Consumers of cmdrunner fakes that don't need
// gate-specific helpers should import cmdtest directly.
package gatetest
