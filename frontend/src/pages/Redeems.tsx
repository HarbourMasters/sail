import { useEffect, useState } from "react";
import { DeleteRedeem, ListRedeems, ListTwitchRewards, SaveRedeem } from "../../wailsjs/go/main/App";
import { config, sail, twitchapi } from "../../wailsjs/go/models";
import BindingEditor from "../components/BindingEditor";
import { BindingDraft, bindingToDraft, bindingToPayload, describeBinding, emptyBinding } from "../lib/drafts";

interface RedeemsProps {
  actions: sail.ActionInfo[];
}

export default function Redeems({ actions }: RedeemsProps) {
  const [rewards, setRewards] = useState<twitchapi.CustomReward[] | null>(null);
  const [redeems, setRedeems] = useState<config.Redeem[]>([]);
  const [loadError, setLoadError] = useState("");
  const [editingRewardId, setEditingRewardId] = useState<string | null>(null);
  const [binding, setBinding] = useState<BindingDraft>(emptyBinding());
  const [saveError, setSaveError] = useState("");

  function refresh() {
    ListRedeems().then(setRedeems);
    ListTwitchRewards()
      .then((live) => {
        setRewards(live);
        setLoadError("");
      })
      .catch((err) => setLoadError(String(err)));
  }

  useEffect(refresh, []);

  function bindingFor(rewardId: string): config.Binding | undefined {
    return redeems.find((r) => r.rewardId === rewardId)?.binding;
  }

  function edit(reward: twitchapi.CustomReward) {
    setEditingRewardId(reward.id);
    const existing = bindingFor(reward.id);
    setBinding(existing ? bindingToDraft(existing) : emptyBinding());
    setSaveError("");
  }

  async function save(reward: twitchapi.CustomReward) {
    try {
      await SaveRedeem(
        config.Redeem.createFrom({
          rewardId: reward.id,
          rewardTitle: reward.title,
          binding: bindingToPayload(binding, actions),
        }),
      );
      setEditingRewardId(null);
      refresh();
    } catch (err) {
      setSaveError(String(err));
    }
  }

  async function clear(rewardId: string) {
    await DeleteRedeem(rewardId);
    if (editingRewardId === rewardId) setEditingRewardId(null);
    refresh();
  }

  return (
    <div className="page">
      <h1>Channel point redeems</h1>
      <p className="page-hint">Map your channel point rewards to in-game effects.</p>

      {loadError && (
        <div className="banner banner-warning">
          Couldn't load rewards from Twitch ({loadError}). Log in on the Dashboard first.
        </div>
      )}

      {rewards && rewards.length === 0 && (
        <p className="empty-hint">No channel-point rewards in these waters yet — create one on Twitch first.</p>
      )}

      <div className="two-column">
        <div className="card list-card">
          <ul className="config-list">
            {(rewards ?? []).map((reward) => {
              const bound = bindingFor(reward.id);
              return (
                <li key={reward.id} className={reward.id === editingRewardId ? "active" : ""}>
                  <button type="button" className="list-item-button" onClick={() => edit(reward)}>
                    <strong>{reward.title}</strong>
                    <span className="list-item-detail">
                      {reward.cost} points{bound ? ` — ${describeBinding(bound)}` : " — not configured"}
                    </span>
                  </button>
                  {bound && (
                    <button type="button" className="btn-icon" onClick={() => clear(reward.id)}>
                      Clear
                    </button>
                  )}
                </li>
              );
            })}
          </ul>
        </div>

        <div className="card">
          {editingRewardId ? (
            <>
              <BindingEditor binding={binding} actions={actions} onChange={setBinding} />
              {saveError && <div className="banner banner-error">{saveError}</div>}
              <button
                type="button"
                onClick={() => {
                  const reward = rewards?.find((r) => r.id === editingRewardId);
                  if (reward) save(reward);
                }}
              >
                Save binding
              </button>
            </>
          ) : (
            <p className="empty-hint">Select a reward on the left to configure what it does in-game.</p>
          )}
        </div>
      </div>
    </div>
  );
}
