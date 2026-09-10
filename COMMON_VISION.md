# Common Vision prototype

ORIGIN-0's new central workspace serves the user's political and societal mission:
collect needs and disagreements, maintain a shared direction, compare institutions,
and connect reviewed proposals to recorded outcomes. Image generation, architecture,
research and technical development remain available as supporting tools.

## Start and try the complete cycle

Run the prototype executable and open **Common Vision**. No model download or GPU
is needed for this workspace. Existing media functions retain their own setup.

1. Select **Load labelled demonstration**. Four synthetic perspectives include a
   resident need, a minority objection, a staff concern and identified bot analysis.
2. Inspect **Common vision**, **12 + 1 values**, **Living manifesto**, **Living
   manuscript** and **Implementation programme**. These are five views of the same
   versioned framework snapshot, with contribution/proposal/outcome IDs and a hash.
3. Select **Review adoption**, read the scope, identify yourself as the local human
   reviewer and record your reason. This adopts a local prototype snapshot only.
4. Open the public decision-response ledger proposal. Compare its alternatives,
   costs, responsibilities, assumptions, metric and stopping rule. Select **Review
   proposal**, then separately **Record experiment authorisation**.
5. Select **Record feedback**. The demonstration fills synthetic results but does
   not save them until you submit. Inspect and save the simulation record.
6. Select **Draft follow-up proposal**. Its objective comes from the recorded next
   change, and its assumptions retain the outcome ID and limitations. It needs a new
   review. The original adopted framework remains unchanged.
7. Add your own contribution with its source type, language, context and permission.
   Enter a factual claim as evidence only with a source reference; the software still
   labels it unverified. Every saved change creates a new working document snapshot.

## Source status and document boundaries

The original 12 + 1 wording, the distinctive meaning of +1, the original manifesto
and the original manuscript were not recovered. The recovered foundation brief
explicitly says they are absent. Slots remain **awaiting-source**. The new starter
vision and generated registers are prototype drafts, never attributed to the missing
original documents. Supplying a value requires exact wording, a source citation and
local human confirmation; +1 also requires its explanation. Earlier framework
snapshots retain their earlier value versions.

A manifesto is currently a generated commitment/priority register. The manuscript
is a detailed source/decision register, not an automatically written full-length book.
Topics are entered explicitly; the prototype does not claim semantic interpretation
or automatic translation. Original text is retained in its submitted language.

## Transparent synthesis and review

- Method `transparent-topic-register-v1` groups normalised topic labels. It does not
  infer agreement between semantically similar sentences.
- Declared importance, urgency and consequence each range from 0 to 3 and have equal
  weight. A topic's displayed priority averages the latest score per distinct human
  actor ID. This is an inspectable prioritisation heuristic, not moral truth or
  factual confidence. Weight editing and sensitivity analysis are future work.
- Posting repeatedly does not add human participants. Actor source type cannot be
  changed. Automated, community/organisation/expert and demonstration records are
  separate. All objections remain in the manuscript even if a later stance changes.
- Actor IDs are entered by the local owner. They are not verified identities; this
  prototype cannot prevent someone creating multiple false IDs. Counts are not
  representative polling, universal agreement or a public mandate.
- Freshness checks reject stale edits and decisions. Adoption/rejection requires a
  named local human, an explicit confirmation and a reason. The reviewed snapshot
  hash is recorded. Technical model calls do not receive an adoption capability.
- Proposal review and experiment authorisation are separate. Authorisation only
  records the owner's declared scope; this prototype does not spend money, publish,
  contact people, run institutional interventions or enforce an external mandate.
- Simulation and reported observation are distinct. Demonstration results must stay
  simulations. A reported result alone establishes neither independent verification
  nor causality. Follow-up proposals retain the uncertainty.

## Storage, recovery and limits

Data is saved atomically to `origin0_data/common-vision.json`, with immutable framework
snapshots and local decisions inside the file. An error loading it disables edits
without overwriting the original. PIN/session protection applies to all new APIs.
Nothing is sent to a model or public service by this module.

**Export recovery JSON** downloads that file. To restore, stop ORIGIN-0, keep a copy
of the existing data folder, place the exported file at
`origin0_data/common-vision.json`, then reopen. Copy the complete `origin0_data`
folder to preserve both mission records and the older modules. Existing general
saved-state ZIPs do not yet embed the new mission file; use this export or a full
folder backup for Common Vision recovery.

Withdrawal excludes a contribution from future synthesis but retains the original
local record and previously adopted snapshots. This is not a complete erasure system
or consent-management service. There is no public network participation endpoint.

The prototype limits contributions/revisions to 500, framework snapshots to 400,
and serialised state to 16 MiB. It rejects further writes at the limit rather than
pruning history silently. Source, model weights and user data remain separate.

## Next concrete step

Supply the exact founding value list, +1 explanation and source documents. Then run
one small, explicitly scoped pilot with authorised human contributions, contributor
correction, sampling/coverage review and a prespecified outcome measure. Broader
community governance, identity assurance, translations and field experiments require
additional design and authorisation. The starter proposal is not political advice
or a statement that this institution is superior to its alternatives.
