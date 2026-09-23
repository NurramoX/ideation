# Ideation

A single-user service that is the one home for ideas, so they stop rotting in scattered folders.

## Language

**Idea**:
A single markdown document with a title, identified by a server-assigned id. Ideas are flat: they have no hierarchy and no location other than the server.
_Avoid_: Note, file, entry, project

**Tag**:
A bare label on an idea, with no key and no value, used for grouping and filtering. An idea carries a set of tags. A tag is not an attribute.
_Avoid_: Category, folder, label

**Attribute**:
A free-form key–value pair on an idea, with exactly one value per key. Status is the only built-in attribute: the only one the service itself gives meaning to.
_Avoid_: Field, property, metadata

**Status**:
The built-in attribute saying where an idea is in its life. Every idea has exactly one: raw (captured, not yet through Review, however much it was discussed beforehand), active (alive, however slowly it moves), done (realised or concluded) or dropped (decided against). Raw and active are open; done and dropped are closed. An idea may move from any status to any other.
_Avoid_: State, stage

**Capture**:
Creating a new idea, either by an agent distilling a discussion into a document or by the user writing it down. A captured idea starts raw unless a status is named. Folding a later discussion into an existing idea is an edit, not a capture.
_Avoid_: Upload, import, save

**Version**:
A counter on an idea that advances with every change. A writer must name the version it last saw, so a change it never saw is not overwritten. It is not history: earlier versions are gone.
_Avoid_: Revision, etag

**Filter**:
An expression over an idea's tags, attributes, text and dates that selects a set of ideas.
_Avoid_: Search, query

**Review**:
Cycling through ideas one at a time to revisit and act on them. Reviewing an idea is itself recorded, even when nothing about the idea changes, so the ideas unseen the longest come up first.
_Avoid_: Browse, triage
