// Local icon components, replacing lucide-vue-next.
// Icon geometry is from Lucide v0.303.0 (ISC licence, https://lucide.dev/license).
// To add an icon, copy its shapes by hand from https://lucide.dev into icon-data.ts
// (keep entries sorted) and export it below.

import { h, type Component, type FunctionalComponent } from 'vue'
import { iconData } from './icon-data'

interface IconProps {
  size?: number | string
  strokeWidth?: number | string
}

// 'Trash2' -> 'trash-2', 'UserPlus' -> 'user-plus' (lucide file naming); used as a stable test hook.
const toKebab = (name: string) =>
  name.replace(/([a-z])([A-Z])/g, '$1-$2').replace(/(?<!\d)([A-Za-z])(\d)/g, '$1-$2').toLowerCase()

function defineIcon(name: string): Component {
  const Icon: FunctionalComponent<IconProps> = (props, { attrs }) =>
    h(
      'svg',
      {
        xmlns: 'http://www.w3.org/2000/svg',
        viewBox: '0 0 24 24',
        fill: 'none',
        stroke: 'currentColor',
        'stroke-width': props.strokeWidth ?? 2,
        'stroke-linecap': 'round',
        'stroke-linejoin': 'round',
        width: props.size ?? 16,
        height: props.size ?? 16,
        'aria-hidden': 'true',
        'data-icon': toKebab(name),
        ...attrs,
      },
      iconData[name].map(([tag, shape]) => h(tag, shape)),
    )
  Icon.props = ['size', 'strokeWidth']
  Icon.displayName = name
  Object.defineProperty(Icon, 'name', { value: name })
  return Icon
}

export const Activity = defineIcon('Activity')
export const AlertCircle = defineIcon('AlertCircle')
export const AlertTriangle = defineIcon('AlertTriangle')
export const Archive = defineIcon('Archive')
export const ArrowDown = defineIcon('ArrowDown')
export const ArrowLeft = defineIcon('ArrowLeft')
export const ArrowRight = defineIcon('ArrowRight')
export const ArrowRightLeft = defineIcon('ArrowRightLeft')
export const ArrowUp = defineIcon('ArrowUp')
export const ArrowUpDown = defineIcon('ArrowUpDown')
export const BarChart = defineIcon('BarChart')
export const BarChart3 = defineIcon('BarChart3')
export const Bell = defineIcon('Bell')
export const Bot = defineIcon('Bot')
export const Braces = defineIcon('Braces')
export const Brain = defineIcon('Brain')
export const Bug = defineIcon('Bug')
export const Building2 = defineIcon('Building2')
export const Calendar = defineIcon('Calendar')
export const Check = defineIcon('Check')
export const CheckCheck = defineIcon('CheckCheck')
export const CheckCircle = defineIcon('CheckCircle')
export const CheckCircle2 = defineIcon('CheckCircle2')
export const CheckSquare = defineIcon('CheckSquare')
export const ChevronDown = defineIcon('ChevronDown')
export const ChevronLeft = defineIcon('ChevronLeft')
export const ChevronRight = defineIcon('ChevronRight')
export const ChevronsLeft = defineIcon('ChevronsLeft')
export const ChevronsRight = defineIcon('ChevronsRight')
export const ChevronsUpDown = defineIcon('ChevronsUpDown')
export const CircleDot = defineIcon('CircleDot')
export const Clock = defineIcon('Clock')
export const Code = defineIcon('Code')
export const Coffee = defineIcon('Coffee')
export const Contact = defineIcon('Contact')
export const Copy = defineIcon('Copy')
export const Download = defineIcon('Download')
export const ExternalLink = defineIcon('ExternalLink')
export const Eye = defineIcon('Eye')
export const EyeOff = defineIcon('EyeOff')
export const Facebook = defineIcon('Facebook')
export const FileSpreadsheet = defineIcon('FileSpreadsheet')
export const FileText = defineIcon('FileText')
export const Filter = defineIcon('Filter')
export const Flag = defineIcon('Flag')
export const GitBranch = defineIcon('GitBranch')
export const Globe = defineIcon('Globe')
export const GripVertical = defineIcon('GripVertical')
export const Hand = defineIcon('Hand')
export const Hash = defineIcon('Hash')
export const Headphones = defineIcon('Headphones')
export const History = defineIcon('History')
export const Home = defineIcon('Home')
export const Image = defineIcon('Image')
export const Info = defineIcon('Info')
export const Key = defineIcon('Key')
export const Layers = defineIcon('Layers')
export const LayoutDashboard = defineIcon('LayoutDashboard')
export const LayoutTemplate = defineIcon('LayoutTemplate')
export const LineChart = defineIcon('LineChart')
export const Link = defineIcon('Link')
export const List = defineIcon('List')
export const ListTree = defineIcon('ListTree')
export const Loader2 = defineIcon('Loader2')
export const Lock = defineIcon('Lock')
export const LogIn = defineIcon('LogIn')
export const LogOut = defineIcon('LogOut')
export const Mail = defineIcon('Mail')
export const MapPin = defineIcon('MapPin')
export const Megaphone = defineIcon('Megaphone')
export const Menu = defineIcon('Menu')
export const MessageCircle = defineIcon('MessageCircle')
export const MessageSquare = defineIcon('MessageSquare')
export const MessageSquareText = defineIcon('MessageSquareText')
export const MessagesSquare = defineIcon('MessagesSquare')
export const Mic = defineIcon('Mic')
export const MicOff = defineIcon('MicOff')
export const Minus = defineIcon('Minus')
export const Monitor = defineIcon('Monitor')
export const Moon = defineIcon('Moon')
export const MoreVertical = defineIcon('MoreVertical')
export const MousePointerClick = defineIcon('MousePointerClick')
export const Music = defineIcon('Music')
export const Network = defineIcon('Network')
export const Paperclip = defineIcon('Paperclip')
export const Pause = defineIcon('Pause')
export const Pencil = defineIcon('Pencil')
export const Phone = defineIcon('Phone')
export const PhoneCall = defineIcon('PhoneCall')
export const PhoneForwarded = defineIcon('PhoneForwarded')
export const PhoneIncoming = defineIcon('PhoneIncoming')
export const PhoneMissed = defineIcon('PhoneMissed')
export const PhoneOff = defineIcon('PhoneOff')
export const PhoneOutgoing = defineIcon('PhoneOutgoing')
export const Play = defineIcon('Play')
export const Plus = defineIcon('Plus')
export const Power = defineIcon('Power')
export const RefreshCw = defineIcon('RefreshCw')
export const Reply = defineIcon('Reply')
export const RotateCcw = defineIcon('RotateCcw')
export const RotateCw = defineIcon('RotateCw')
export const Save = defineIcon('Save')
export const Scale = defineIcon('Scale')
export const ScrollText = defineIcon('ScrollText')
export const Search = defineIcon('Search')
export const Send = defineIcon('Send')
export const Settings = defineIcon('Settings')
export const Settings2 = defineIcon('Settings2')
export const Shield = defineIcon('Shield')
export const ShieldCheck = defineIcon('ShieldCheck')
export const SkipForward = defineIcon('SkipForward')
export const Smartphone = defineIcon('Smartphone')
export const Smile = defineIcon('Smile')
export const SmilePlus = defineIcon('SmilePlus')
export const Sparkles = defineIcon('Sparkles')
export const Star = defineIcon('Star')
export const StickyNote = defineIcon('StickyNote')
export const StopCircle = defineIcon('StopCircle')
export const Store = defineIcon('Store')
export const Sun = defineIcon('Sun')
export const Tag = defineIcon('Tag')
export const Tags = defineIcon('Tags')
export const TestTube2 = defineIcon('TestTube2')
export const TextCursorInput = defineIcon('TextCursorInput')
export const Ticket = defineIcon('Ticket')
export const Trash2 = defineIcon('Trash2')
export const TrendingDown = defineIcon('TrendingDown')
export const TrendingUp = defineIcon('TrendingUp')
export const Type = defineIcon('Type')
export const Undo2 = defineIcon('Undo2')
export const Upload = defineIcon('Upload')
export const User = defineIcon('User')
export const UserCog = defineIcon('UserCog')
export const UserMinus = defineIcon('UserMinus')
export const UserPlus = defineIcon('UserPlus')
export const UserX = defineIcon('UserX')
export const Users = defineIcon('Users')
export const Variable = defineIcon('Variable')
export const Video = defineIcon('Video')
export const Volume2 = defineIcon('Volume2')
export const Wallet = defineIcon('Wallet')
export const Webhook = defineIcon('Webhook')
export const Workflow = defineIcon('Workflow')
export const X = defineIcon('X')
export const XCircle = defineIcon('XCircle')
export const Zap = defineIcon('Zap')
export const ZoomIn = defineIcon('ZoomIn')
export const ZoomOut = defineIcon('ZoomOut')
export const CalendarIcon = defineIcon('Calendar')
export const ChevronLeftIcon = defineIcon('ChevronLeft')
export const ChevronRightIcon = defineIcon('ChevronRight')
export const Edit3 = defineIcon('PenLine')
export const FileIcon = defineIcon('File')
export const Grid3X3 = defineIcon('Grid3x3')
export const UserCircle = defineIcon('CircleUser')
