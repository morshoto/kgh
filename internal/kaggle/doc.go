// Package kaggle contains the Kaggle CLI adapter layer for kgh.
//
// The package is intentionally thin. It validates environment-based credentials,
// invokes the official kaggle binary with timeouts, captures process output, and
// returns typed errors that higher layers can surface cleanly. Workflow-specific
// commands such as kernel push, polling, download, and submission are added on
// top of this foundation in later steps.
package kaggle
