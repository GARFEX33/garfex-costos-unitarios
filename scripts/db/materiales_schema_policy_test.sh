#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
UP="$ROOT/migrations/000002_materiales.up.sql"

require() {
  pattern=$1
  file=$2
  grep -Eiq "$pattern" "$file" || {
    printf 'missing schema contract: %s\n' "$pattern" >&2
    exit 1
  }
}

require "GRANT SELECT" "$UP"
require "REVOKE ALL ON" "$UP"
require "GRANT SELECT ON" "$UP"
require "GRANT SELECT, INSERT, UPDATE, DELETE ON" "$UP"

if awk '
/^GRANT SELECT, INSERT, UPDATE, DELETE ON$/ { in_dml = 1; next }
in_dml && /public\.(material_categories|material_families|unit_definitions|family_unit_policies|attribute_definitions|family_attributes|attribute_options)/ { found = 1 }
in_dml && /TO garfex_app;/ { in_dml = 0 }
END { exit found ? 0 : 1 }
' "$UP"; then
  printf '%s\n' 'catalog tables must not receive runtime DML privileges' >&2
  exit 1
fi

if grep -Eiq "attribute_options.*voltage|voltage.*attribute_options|WHERE d\.code = 'voltage'|300 V|600 V|1 kV|5 kV|15 kV|25 kV|35 kV" "$UP"; then
  printf '%s\n' 'voltage must be represented as a quantity, not an AttributeOption seed' >&2
  exit 1
fi

require "voltage.*QUANTITY.*VOLTAGE|voltage.*VOLTAGE.*QUANTITY" "$UP"
require "quantity_value.*quantity_unit_id" "$UP"

if grep -Eiq 'unit_cost|price|brand|provider|sku|presentation' "$UP"; then
  printf 'commercial or cost column found in Materials Master schema\n' >&2
  exit 1
fi

if grep -Eiq "conduit_type|nominal_diameter|ACERO_GALVANIZADO|WHERE d\.code = 'material'" "$UP"; then
  printf 'old provisional TUBERIAS attributes or options found in Materials Master schema\n' >&2
  exit 1
fi

printf '%s\n' 'materials schema policy checks passed'
