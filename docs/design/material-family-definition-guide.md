# Material Family Definition Guide

This guide defines how a family is added to Materials Master v2 without putting family rules into the domain engine.

## Quick path

1. Create a controlled `MaterialCategory` and `MaterialFamily` code.
2. Add controlled `UnitDefinition` entries and `FamilyUnitPolicy` rows. Mark allowed and suggested units; never add an identity flag.
3. Define typed `AttributeDefinition` entries and assign them with `FamilyAttribute` modes.
4. Add `AttributeOption` values and bounded `AttributeRule` predicates for conditional behavior.
5. Validate a material, select its mandatory `NaturalUnit`, and build identity only from participating attributes.

## Rules

| Concern | Family definition rule |
|---|---|
| NaturalUnit | Required, explicitly selected, allowed by family policy, stored on Material, never identity. |
| Identity | Family code plus sorted canonical values of identity-participating attributes. |
| Values | Use controlled options, integer, decimal, quantity, boolean, or controlled text. |
| Modes | Use `REQUIRED`, `OPTIONAL`, `CONDITIONAL`, or `FORBIDDEN`. |
| Conditions | Use a small predicate over an already validated typed value; do not add a generic rules engine. |
| Commercial data | Keep price, supplier, brand, presentation, package size, and commercial length outside Material. |
| Duplicate detection | Compare exact canonical identity keys only. Do not use display names, similarity, or equivalence. |

## CONDUCTORES catalog reference

The approved initial catalog is represented by `NewMaterialsCatalog` as data:

- Family: `CONDUCTORES`.
- NaturalUnit: `M` allowed and suggested.
- Required identity options: `conductor_material` (`COBRE`, `ALUMINIO`), `gauge` (`14`, `12`, `10`, `8`, `6`, `4`, `2`, `1`, `1/0`, `2/0`, `3/0`, `4/0` AWG), and `insulation` (`DESNUDO`, `THW`, `THW-LS`, `THHN`, `THHN/THWN-2`, `XHHW-2`, `RHH/RHW-2`).
- Conditional color options: `NEGRO`, `BLANCO`, `ROJO`, `AZUL`, `VERDE`.
- Conditional voltage quantities: `300 V`, `600 V`, `1 kV`, `5 kV`, `15 kV`, `25 kV`, `35 kV`.

The catalog expresses the following bounded rules without a conductor-specific engine branch:

- `insulation = DESNUDO` makes color and voltage forbidden/not applicable.
- Any other insulation makes color and voltage required and identity-participating.
- Empty insulation is invalid; it is not an alias for `DESNUDO`.

NaturalUnit changes management metadata only. For example, the same technical conductor selected with another allowed management unit would retain the same identity key; technical changes to gauge, insulation, color, or voltage produce a different key.
