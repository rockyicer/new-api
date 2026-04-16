# Upstream UI Alignment Design

## Context

This design covers the deferred presentation-layer sync after Batch A and Batch B upstream feature integration. The target is to move selected UI areas closer to upstream while preserving the existing JustAPI branding shell.

## Approved Direction

- `layout / dashboard` should align with upstream.
- `branding` must remain JustAPI.
- `ratio settings` can move to the upstream `ModelPricingCombined` structure in one shot.

## Branding Boundary

The following elements must remain local JustAPI behavior and content:

- Top navigation brand name and entry structure
- Footer copy and links
- `About / Contact / Docs` pages and navigation
- Logo and system name

## Chosen Approach

Use a "brand shell + upstream content core" approach:

- Keep the local branding shell in `HeaderBar`, `Footer`, branding sync, and the brand-specific pages.
- Pull upstream structure and interaction updates into `dashboard`, `ratio settings`, and layout polish.
- Split execution into three batches so each batch can be validated and merged independently.

## Execution Batches

### UI-1 Dashboard

Bring the dashboard order, charts, admin analytics placement, and API info panel behavior fully in line with upstream.

Primary upstream commits:

- `606a4eee`
- `77897a81`
- `aafbd788`
- `b2dd4acc` if the current flicker is still reproducible

### UI-2 Ratio Settings

Switch to the upstream `ModelPricingCombined` structure in one shot and accept the page-level locale refresh needed for that move.

Primary upstream commits or transplant sources:

- `dc83c4af`
- `78e4cb3c`
- `c2006093`

### UI-3 Layout Polish

Adopt more upstream layout and footer styling details while preserving the JustAPI footer content and route structure.

Primary upstream commits:

- `310d618a`
- `b2dd4acc` if not already taken in UI-1

Explicitly excluded:

- `7399e472` because it rewrites `EditChannelModal` and exceeds the allowed scope.

## Validation Strategy

Each batch must be validated before merge-back:

1. Resolve all conflicts and confirm no unresolved files remain.
2. Run the batch-relevant checks.
3. Run `bun run build` in `web/`.
4. Merge back to `rockyicer` immediately after the batch passes.

## Merge Policy

Each successful batch should be merged back to `rockyicer` independently to keep rollback scope small.
