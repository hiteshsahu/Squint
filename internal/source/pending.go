package source

// Pending is a human translation of a Slurm pending Reason code.
type Pending struct {
	Plain      string // what's actually happening, in plain English
	Suggestion string // the thing the user can do about it (may be empty)
}

// Explain turns squeue's cryptic Reason field into something a human can act
// on. This is one of squint's two flagship L0 features — the other is the
// idle-GPU heatmap. New codes are cheap to add; keep them short and actionable.
func Explain(reason string) Pending {
	switch reason {
	case "Resources":
		return Pending{
			"Waiting for free GPUs/nodes to run on.",
			"Nothing's wrong — ask for fewer GPUs or a shorter time limit to slot in sooner.",
		}
	case "Priority":
		return Pending{
			"Higher-priority jobs are ahead of you in line.",
			"It starts when the queue clears; a higher QOS is the only real shortcut.",
		}
	case "QOSMaxGRESPerUser":
		return Pending{
			"You've hit your GPU quota for this QOS.",
			"Wait for one of your running jobs to finish, or submit to a QOS/partition with more GPU headroom.",
		}
	case "AssocMaxGRESPerUser", "AssocGrpGRES":
		return Pending{
			"Your account/association has hit its GPU limit.",
			"Another job under your account must finish first, or ask an admin to raise the cap.",
		}
	case "QOSMaxJobsPerUserLimit", "MaxJobsPerUser":
		return Pending{
			"You already have the max number of jobs running for this QOS.",
			"Let a running job complete before this one can start.",
		}
	case "ReqNodeNotAvail", "ReqNodeNotAvail(Unavailable)":
		return Pending{
			"A node you requested is down, draining, or reserved.",
			"Drop the explicit -w/--nodelist request, or pick a partition whose nodes are up.",
		}
	case "Dependency":
		return Pending{
			"Waiting on another job to finish first.",
			"Check the dependency — if that job failed, this one may never start.",
		}
	case "PartitionTimeLimit":
		return Pending{
			"Your time limit exceeds what this partition allows.",
			"Lower --time, or submit to a partition with a longer max wall time.",
		}
	case "Licenses", "licenses":
		return Pending{
			"Waiting on a software license to free up.",
			"Nothing to change — a license-holding job has to release first.",
		}
	case "BeginTime":
		return Pending{
			"Held until its scheduled start time.",
			"You set --begin in the future; it'll start then.",
		}
	case "AssociationJobLimit", "AssocGrpJobsLimit":
		return Pending{
			"Your account has too many jobs queued or running.",
			"Let some of your account's jobs drain first.",
		}
	default:
		if reason == "" || reason == "None" {
			return Pending{"Being scheduled…", ""}
		}
		return Pending{reason, "Uncommon reason — run `scontrol show job <id>` for the full story."}
	}
}
