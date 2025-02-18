package p9kit

var MessageTypes = map[int]string{
	7:   "Rlerror",
	8:   "Tstatfs",
	9:   "Rstatfs",
	12:  "Tlopen",
	13:  "Rlopen",
	14:  "Tlcreate",
	15:  "Rlcreate",
	16:  "Tsymlink",
	17:  "Rsymlink",
	18:  "Tmknod",
	19:  "Rmknod",
	20:  "Trename",
	21:  "Rrename",
	22:  "Treadlink",
	23:  "Rreadlink",
	24:  "Tgetattr",
	25:  "Rgetattr",
	26:  "Tsetattr",
	27:  "Rsetattr",
	30:  "Txattrwalk",
	31:  "Rxattrwalk",
	32:  "Txattrcreate",
	33:  "Rxattrcreate",
	40:  "Treaddir",
	41:  "Rreaddir",
	50:  "Tfsync",
	51:  "Rfsync",
	52:  "Tlock",
	53:  "Rlock",
	54:  "Tgetlock",
	55:  "Rgetlock",
	70:  "Tlink",
	71:  "Rlink",
	72:  "Tmkdir",
	73:  "Rmkdir",
	74:  "Trenameat",
	75:  "Rrenameat",
	76:  "Tunlinkat",
	77:  "Runlinkat",
	100: "Tversion",
	101: "Rversion",
	102: "Tauth",
	103: "Rauth",
	104: "Tattach",
	105: "Rattach",
	108: "Tflush",
	109: "Rflush",
	110: "Twalk",
	111: "Rwalk",
	116: "Tread",
	117: "Rread",
	118: "Twrite",
	119: "Rwrite",
	120: "Tclunk",
	121: "Rclunk",
	122: "Tremove",
	123: "Rremove",
	124: "Tflushf",
	125: "Rflushf",
	126: "Twalkgetattr",
	127: "Rwalkgetattr",
	128: "Tucreate",
	129: "Rucreate",
	130: "Tumkdir",
	131: "Rumkdir",
	132: "Tumknod",
	133: "Rumknod",
	134: "Tusymlink",
	135: "Rusymlink",
}
