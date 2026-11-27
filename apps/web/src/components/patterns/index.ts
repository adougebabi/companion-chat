// Fluctlight business UI patterns. Pick from here before writing new layout.
// Catalogue and rules: .impeccable.md; live states: apps/web/ui-lab (?fixture=gallery).
export { PageContainer, PageSection } from "./page";
export { DetailSheet, EditSheet } from "./sheets";
export { FormSection, FormField, FormActions } from "./form";
export { AsyncTaskStatus, type AsyncTaskState } from "./status";
export { DataState, RecordList, RecordListItem, DetailFields } from "./data";
export { ResponsiveSheet } from "@/components/ui/responsive-sheet";
export { ResponsiveTabsList, ResponsiveTabs, type ResponsiveTabItem } from "@/components/ui/responsive-tabs";
export { Feedback, EmptyState, type FeedbackKind } from "@/react-app/shared/Presentation";
export { default as LoadingState } from "@/react-app/shared/LoadingState";

export { MobileNavigation, type MobileNavigationItem } from "./mobile-navigation";
