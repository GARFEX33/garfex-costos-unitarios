#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
UP="$ROOT/migrations/000002_materiales.up.sql"
DOWN="$ROOT/migrations/000002_materiales.down.sql"

require() {
  pattern=$1
  file=$2
  grep -Eiq "$pattern" "$file" || {
    printf 'missing schema contract: %s\n' "$pattern" >&2
    exit 1
  }
}

for table in material_categories material_families unit_definitions family_unit_policies \
  attribute_definitions family_attributes attribute_options attribute_option_relations materiales \
  material_attribute_values; do
  require "CREATE TABLE.*$table" "$UP"
done

require 'identity_key.*NOT NULL' "$UP"
require 'UNIQUE.*family_id.*identity_key|UNIQUE.*identity_key.*family_id' "$UP"
require 'UNIQUE \(id, family_id, definition_id\)' "$UP"
require 'UNIQUE \(id, family_id\)' "$UP"
require 'FOREIGN KEY \(material_id, family_id\)' "$UP"
require 'REFERENCES public\.materiales\(id, family_id\)' "$UP"
require 'FOREIGN KEY \(family_attribute_id, family_id, attribute_definition_id\)' "$UP"
require 'REFERENCES public\.family_attributes\(id, family_id, definition_id\)' "$UP"
require 'FOREIGN KEY \(attribute_definition_id, option_code\)' "$UP"
require 'REFERENCES public\.attribute_options\(attribute_definition_id, code\)' "$UP"
require 'CREATE TABLE.*attribute_option_relations' "$UP"
require 'attribute_option_relations.*from_attribute_definition_id' "$UP"
require 'attribute_option_relations.*to_attribute_definition_id' "$UP"
require 'UNIQUE.*from_attribute_definition_id.*to_attribute_definition_id' "$UP"
require 'CREATE FUNCTION public\.validate_material_attribute_value' "$UP"
require 'CREATE TRIGGER material_attribute_values_validate_type' "$UP"
require 'fa\.family_id = NEW\.family_id' "$UP"
require 'fa\.definition_id = NEW\.attribute_definition_id' "$UP"
require 'CONTROLLED_OPTION values require an official option' "$UP"
require 'only CONTROLLED_OPTION values may reference an option' "$UP"
require "value_state.*NOT NULL" "$UP"
require "NOT_APPLICABLE" "$UP"
require "DESNUDO" "$UP"
require "THHN/THWN-2" "$UP"
require "4/0 AWG" "$UP"
require "DROP TABLE.*material_attribute_values" "$DOWN"
require "DROP TRIGGER IF EXISTS material_attribute_values_validate_type" "$DOWN"
require "DROP FUNCTION.*validate_material_attribute_value" "$DOWN"

printf '%s\n' 'materials schema structure checks passed'
