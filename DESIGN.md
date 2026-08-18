# Transactional Email Design System

## Direction

**Execution Timeline** treats every launchctl email as an operational handoff, not a stack of notification cards. The recipient should understand what happened, which resource changed, and the next useful action in one uninterrupted scan path. Completed context stays visible but quiet; current state and evidence carry the emphasis.

- Direction seed: `47c85962`
- Surface: all transactional HTML email rendered by `internal/pkg/mail/templates`
- Reference stress case: deployment failure
- Approved shape: one continuous, bordered 600px white execution sheet on a zinc canvas
- Anti-pattern: nested card stacks or detached dashboard widgets

## Visual Language

### Typography

- Interface and narrative: **Plus Jakarta Sans**, falling back to Apple system, Segoe UI, Roboto, Helvetica, and Arial.
- Operational data: **JetBrains Mono**, falling back to SFMono, Consolas, Courier New, Liberation Mono, Menlo, and monospace.
- Mono is reserved for routes, state labels, identifiers, measurements, metadata, actions, and execution output.
- Titles use tight tracking (`-0.045em`); labels are compact, uppercase, and letter-spaced.

### Core tokens

| Role | Token |
| --- | --- |
| Canvas | `#f4f4f5` |
| Sheet | `#ffffff` |
| Primary ink | `#09090b` |
| Strong ink / action | `#18181b` |
| Heading ink | `#27272a` |
| Body ink | `#52525b` |
| Muted ink | `#71717a` |
| Quiet ink | `#a1a1aa` |
| Rail / outer border | `#d4d4d8` |
| Divider | `#e4e4e7` |
| Evidence surface | `#fafafa` |
| Error bright / node | `#ef4444` |
| Error ink / action | `#b91c1c` |
| Error deep ink | `#991b1b` |
| Error tint / rule | `#fef2f2` / `#fecaca` |
| Success node / action | `#16a34a` / `#15803d` |
| Warning node / label | `#d97706` / `#a16207` |

Color is semantic, never decorative. Neutral events use zinc; success, warning, and error tones identify state through both color and a written label. Error output also gains a tinted row and marker.

### Geometry and rhythm

- Sheet: `max-width: 600px`, white, fixed-layout, with a `1px #d4d4d8` border.
- Outer spacing: 36px × 12px desktop; 12px × 6px below 620px.
- Header: 22px × 28px with a hairline lower divider.
- Generic content inset: 36px 28px 10px; incident content uses a 52px rail and 28–36px copy inset.
- Timeline rail: a continuous `1px #d4d4d8` right border.
- State nodes: square, not circular. The current state is 9–10px with a 2px white keyline; action and evidence nodes are smaller.
- Panels and tables use hairline top/bottom rules and tonal fills, not enclosing card borders.
- Footer: 18px × 28px with a hairline top divider.

## Anatomy

Every message follows this reading order:

1. Hidden preheader, when supplied.
2. Header with the `launchctl` wordmark and compact product route.
3. Current state node, explicit state label, and decisive event title.
4. Ordered blocks in the exact sequence supplied by the builder.
5. Optional fallback subcopy.
6. Transaction footer.

The builder preserves block order because sequence is meaning. Available blocks are `intro`, `content`, `action`, `panel`, `table`, and `outro`. Content blocks continue the same rail; they do not become independent cards.

## Variants

### Generic timeline

Use for lightweight authentication, invitations, lifecycle receipts, provisioning notices, monitoring notices, and other standard transactions.

- Default route: `lctl / notification`.
- Default state: `EVENT` / neutral.
- One primary state node introduces the greeting or title.
- Actions add a solid square node and a mono button.
- Panels and tables add outlined evidence nodes.
- Supported semantic tones: neutral, success, warning, error.

### Incident timeline

Use the private `incident` layout for evidence-heavy operational failures. Deployment failure is the canonical implementation.

- Header route is fixed to `lctl / deploy`.
- The timeline starts with failure state, heading, site, and optional server.
- Optional failure summary follows in a pale red, ruled evidence band.
- Run context presents commit hash/message and execution metadata.
- The primary action precedes execution output.
- Output is a numbered mono trace; failure and cleanup lines remain distinguishable without hiding surrounding evidence.
- The incident is still one continuous sheet and rail, never a stack of cards.

## Actions and content

- Primary buttons are zinc; success and error buttons may use their semantic dark tone.
- Button labels use JetBrains Mono and a right arrow; the destination remains a normal meaningful link.
- Markdown supports structured prose and GFM conveniences, but raw HTML is disabled.
- Arbitrary callers cannot inject trusted HTML: pre-rendered `template.HTML` content and layout selection remain private builder operations.
- Dynamic values, URLs, labels, table cells, metadata, and output are rendered through `html/template` contextual escaping.
- Long URLs, hashes, resource names, summaries, and output wrap with `overflow-wrap:anywhere`, `word-break`, or `pre-wrap` as appropriate.
- Every HTML message must retain a complete plain-text alternative with the same ordered facts and action URL.

## Email-client rules

- Structure layout with presentation tables, explicit widths, `cellpadding="0"`, and `cellspacing="0"`.
- Put critical layout, typography, borders, colors, and wrapping inline; head CSS is progressive enhancement only.
- Use explicit `bgcolor` fallbacks for important surfaces and actions.
- Wrap the 600px sheet in an MSO conditional fixed-width table for Outlook.
- Buttons use a table cell, `mso-padding-alt`, solid background, and matching anchor borders.
- Keep `table-layout:fixed` where long technical content could expand the sheet.
- Do not depend on border radius, flexbox, grid, pseudo-elements, remote font loading, or JavaScript.
- Preserve semantic document order even when CSS is stripped.

## Responsive behavior

- At 620px and below, outer padding compresses, header/footer use 20px sides, and timeline/incident content uses 18px sides.
- At 420px and below, route text drops to 9px, titles to 25/32px, timeline copy inset to 18px, and buttons become centered full-width blocks.
- Detail labels may contract to 88px; evidence/output columns keep fixed marker gutters while code wraps.
- Mobile adaptation changes spacing and wrapping, not the event order or available evidence.

## Guardrails

- Preserve business facts, destination URLs, state meaning, and plain-text parity.
- Keep status cues textual and high-contrast; color is supplemental.
- Keep the first viewport focused on event, resource, current state, and action.
- Avoid decorative gradients, shadows, illustrations, rounded containers, or multiple competing CTAs.
- Never introduce nested card stacks. Extend the timeline with another ordered state or ruled evidence section.
- New templates inherit the shared shell; specialized variants remain private and require a real information-shape need.
