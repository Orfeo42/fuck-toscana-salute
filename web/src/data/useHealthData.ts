import { useContext } from "react";

import { HealthDataContext, type HealthData } from "./healthContext";

export function useHealthData(): HealthData {
  const value = useContext(HealthDataContext);
  if (value === undefined) {
    throw new Error("useHealthData must be used inside HealthDataProvider");
  }
  return value;
}
