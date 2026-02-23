// User
export interface User {
  id: number;
  email: string;
  displayName: string;
  createdAt: string;
  updatedAt: string;
}

// Stitch
export interface Stitch {
  id: number;
  abbreviation: string;
  name: string;
  description: string;
  category: string;
  isCustom: boolean;
  userId: number | null;
  createdAt: string;
}

// Pattern
export type PatternType = 'round' | 'row';

export interface StitchEntry {
  id: number;
  instructionGroupId: number;
  sortOrder: number;
  stitchId: number;
  count: number;
  intoStitch: string;
  repeatCount: number;
}

export interface InstructionGroup {
  id: number;
  patternId: number;
  sortOrder: number;
  label: string;
  repeatCount: number;
  stitchEntries: StitchEntry[];
  expectedCount: number | null;
  notes: string;
}

export interface Pattern {
  id: number;
  userId: number;
  name: string;
  description: string;
  patternType: PatternType;
  hookSize: string;
  yarnWeight: string;
  difficulty: string;
  instructionGroups: InstructionGroup[];
  createdAt: string;
  updatedAt: string;
}

// Work Session
export interface WorkSession {
  id: number;
  patternId: number;
  userId: number;
  currentGroupIndex: number;
  currentGroupRepeat: number;
  currentStitchIndex: number;
  currentStitchRepeat: number;
  currentStitchCount: number;
  status: 'active' | 'paused' | 'completed';
  startedAt: string;
  lastActivityAt: string;
  completedAt: string | null;
}

// Session Progress (computed by backend)
export interface GroupProgress {
  label: string;
  repeatCount: number;
  currentRepeat: number;
  status: 'completed' | 'current' | 'upcoming';
  completedInGroup: number;
  totalInGroup: number;
}

export interface SessionProgress {
  completedStitches: number;
  totalStitches: number;
  percentage: number;
  groupLabel: string;
  groupRepeatInfo: string;
  currentAbbr: string;
  currentName: string;
  prevAbbr: string;
  nextAbbr: string;
  groups: GroupProgress[];
}

// Pattern Image
export interface PatternImage {
  id: number;
  instructionGroupId: number;
  filename: string;
  contentType: string;
  size: number;
  sortOrder: number;
  createdAt: string;
}

// Dashboard
export interface DashboardData {
  activeSessions: WorkSession[];
  completedSessions: WorkSession[];
  patternNames: Record<string, string>;
  totalCompleted: number;
}

// API Error response
export interface ApiError {
  error: string;
}

// Session view response (combined session + pattern + progress)
export interface SessionViewData {
  session: WorkSession;
  pattern: Pattern;
  progress: SessionProgress;
}
