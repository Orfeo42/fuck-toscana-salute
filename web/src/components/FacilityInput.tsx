import { Autocomplete, Loader, Stack, Text } from "@mantine/core";
import type { ComboboxItem } from "@mantine/core";
import { useDebouncedValue } from "@mantine/hooks";
import { useEffect, useMemo, useState } from "react";
import type { JSX } from "react";

import { api } from "../api/client";
import type { components } from "../api/schema";

type FacilityBody = components["schemas"]["FacilityBody"];

interface Props {
  label: string;
  value: string;
  onChange: (value: string) => void;
  description?: string;
}

interface Results {
  query: string;
  facilities: FacilityBody[];
}

const minChars = 2;
const debounceMs = 250;
const attribution =
  "Elenco strutture: Ministero della Salute (IODL 2.0), Regione Toscana (CC-BY 4.0)";

function optionValue(facility: FacilityBody): string {
  return `${facility.name} · ${facility.comune} · ${facility.address}`;
}

export function FacilityInput({
  label,
  value,
  onChange,
  description,
}: Props): JSX.Element {
  const [query, setQuery] = useState("");
  const [debounced] = useDebouncedValue(query, debounceMs);
  const [results, setResults] = useState<Results>({
    query: "",
    facilities: [],
  });
  const text = debounced.trim();
  const searching = text.length >= minChars;

  useEffect(() => {
    if (text.length < minChars) {
      return;
    }
    const controller = new AbortController();
    const search = async (): Promise<void> => {
      try {
        const { data } = await api.POST("/api/facilities/search", {
          body: { query: text },
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }
        setResults({ query: text, facilities: data?.facilities ?? [] });
      } catch {
        if (!controller.signal.aborted) {
          setResults({ query: text, facilities: [] });
        }
      }
    };
    void search();
    return () => {
      controller.abort();
    };
  }, [text]);

  const facilities = useMemo(
    () => (searching ? results.facilities : []),
    [searching, results],
  );
  const byValue = useMemo(
    () =>
      new Map(facilities.map((facility) => [optionValue(facility), facility])),
    [facilities],
  );
  const data: ComboboxItem[] = useMemo(
    () =>
      facilities.map((facility) => ({
        value: optionValue(facility),
        label: facility.name,
      })),
    [facilities],
  );
  const loading = searching && results.query !== text;

  const handleChange = (next: string): void => {
    const picked = byValue.get(next);
    if (picked !== undefined) {
      onChange(picked.name);
      return;
    }
    setQuery(next);
    onChange(next);
  };

  return (
    <Stack gap={4}>
      <Autocomplete
        label={label}
        {...(description === undefined ? {} : { description })}
        value={value}
        onChange={handleChange}
        data={data}
        filter={({ options }) => options}
        rightSection={loading ? <Loader size="xs" /> : null}
        renderOption={({ option }) => {
          const facility = byValue.get(option.value);
          if (facility === undefined) {
            return null;
          }
          return (
            <div>
              <Text size="sm">{facility.name}</Text>
              <Text size="xs" c="dimmed">
                {facility.comune} ({facility.provincia})
                {facility.address === "" ? "" : ` · ${facility.address}`}
              </Text>
            </div>
          );
        }}
      />
      <Text size="xs" c="dimmed">
        {attribution}
      </Text>
    </Stack>
  );
}
