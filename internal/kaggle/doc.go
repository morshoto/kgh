// Package kaggle contains the Kaggle adapter layer for kgh.
//
// Adapter is the workflow-facing contract used by higher layers.
// Client is a lower-level Kaggle CLI executor that concrete adapters can build on.
//
// The package remains intentionally thin. It resolves Kaggle credentials,
// stages local kernel bundles, delegates external command execution to
// internal/execx, polls kernel status until a terminal state is reached, and
// returns typed errors that higher layers can surface cleanly.
package kaggle
