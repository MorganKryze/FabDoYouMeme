/**
 * Centralised barrel export for all Lucide icons used in the app.
 *
 * Every component imports icons from this file — never directly from
 * `lucide-svelte`. This gives us:
 *   1. A single audit point for which icons the app uses
 *   2. A single file to edit if we ever swap libraries
 *   3. The ability to rename icons semantically without rippling changes
 *      (e.g. `LobbyIcon = Home`) once brand vocabulary locks in
 *
 * When a new screen needs an icon, add the import here first, then
 * import from `$lib/icons` in the component.
 */
export {
  // --- Navigation & primary routes ---
  Play,
  Home,
  Sparkles,
  User,
  Shield,
  Package,
  Users,
  Mail,
  Hash,
  Trophy,
  Wrench,

  // --- Actions ---
  Plus,
  Send,
  Link2,
  Share2,
  Check,
  CheckCircle,
  Copy,
  Trash,
  Trash2,
  Save,
  Upload,
  Download,
  Edit,
  Edit2,
  Search,
  Shuffle,
  LogOut,
  LogIn,
  Settings,
  UserPlus,
  UserX,
  Gavel,
  Flag,
  Ban,

  // --- Marketing & security ---
  Lock,
  Server,
  Gamepad2,

  // --- Status & info ---
  Info,
  HelpCircle,
  ListChecks,
  Clock,
  FileText,
  Sliders,
  Heart,
  Bell,
  XCircle,
  X,
  PartyPopper,
  IdCard,
  AlertTriangle,

  // --- Theme ---
  Sun,
  Sunrise,
  Sunset,
  Moon,

  // --- Studio / editor ---
  Crop,
  RotateCw,
  RotateCcw,
  Type,
  Bold,
  Image as ImageIcon,

  // --- Arrows & chevrons ---
  ArrowLeft,
  ArrowRight,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
} from 'lucide-svelte';
