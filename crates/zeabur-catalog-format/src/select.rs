//! Choosing components [installers]: the defaults of a profile, what a person enables and
//! disables, what the cluster cannot run, and the install order.
//!
//! [installers]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/installers.mdx#choose-the-components

use std::collections::{BTreeMap, BTreeSet};

use serde::{Deserialize, Serialize};

use crate::api::{ComponentFile, Index, TestedOn};
use crate::common::{Distribution, Mode, PerProfile, Profile};
use crate::component::{Component, Provides};
use crate::order::{self, OrderError};

/// What selection reads of a component, with the `testedOn` and `installOrder` worked out.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Candidate {
    /// Its id.
    pub id: String,
    /// The capability it provides.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provides: Option<Provides>,
    /// Its mode on each profile.
    pub defaults: PerProfile<Mode>,
    /// The distributions it runs on.
    pub distributions: Vec<Distribution>,
    /// Where it passed its test.
    pub tested_on: Vec<TestedOn>,
    /// What installing it installs; a profile left out means the component alone.
    #[serde(default)]
    pub install_order: InstallOrders,
}

/// The install order on each profile.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub struct InstallOrders {
    /// On `single-node`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub single_node: Option<Vec<String>>,
    /// On `multi-node`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub multi_node: Option<Vec<String>>,
}

impl InstallOrders {
    /// The order on `profile`.
    #[must_use]
    pub fn get(&self, profile: Profile) -> Option<&[String]> {
        match profile {
            Profile::SingleNode => self.single_node.as_deref(),
            Profile::MultiNode => self.multi_node.as_deref(),
        }
    }
}

impl Candidate {
    /// The candidate of `file`, with its `testedOn` from `index` and its install order among
    /// `components`. `None` when this reader cannot install it ([`Component::known`]), so it is
    /// not offered.
    ///
    /// # Errors
    ///
    /// A `requires` entry names nothing, or the requirements loop.
    pub fn new<C: AsRef<Component>>(
        file: &ComponentFile,
        index: &Index,
        components: &BTreeMap<String, C>,
    ) -> Result<Option<Self>, OrderError> {
        if !file.declaration.known() {
            return Ok(None);
        }
        let order = |p| order::install_order(components, &file.id, p);
        Ok(Some(Self {
            id: file.id.clone(),
            provides: file.declaration.provides.clone(),
            defaults: file.declaration.defaults,
            distributions: file.declaration.distributions.clone(),
            tested_on: file.tested_on(index),
            install_order: InstallOrders {
                single_node: order(Profile::SingleNode)?,
                multi_node: order(Profile::MultiNode)?,
            },
        }))
    }

    fn capability(&self) -> Option<&str> {
        self.provides.as_ref()?.capability.as_deref()
    }

    fn tested(&self, config: &Config) -> bool {
        self.tested_on.iter().any(|t| {
            t.distribution == config.distribution && t.platforms.contains(&config.platform)
        })
    }

    fn left_out(&self, config: &Config) -> Option<LeftOut> {
        if !self.distributions.contains(&config.distribution) {
            Some(LeftOut::Distributions {
                distributions: self.distributions.clone(),
            })
        } else if self.defaults.mode(config.profile) == Mode::Unsupported {
            Some(LeftOut::Unsupported {
                profile: config.profile,
            })
        } else {
            None
        }
    }
}

/// The cluster and the lists of the installer and the person.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Config {
    /// The profile.
    pub profile: Profile,
    /// The distribution.
    pub distribution: Distribution,
    /// The platform of the nodes, such as `linux/amd64`.
    pub platform: String,
    /// What the installer adds to the defaults.
    #[serde(default)]
    pub add: Vec<String>,
    /// What the person turns on, by id or capability.
    #[serde(default)]
    pub enable: Vec<String>,
    /// What the person turns off, by id or capability.
    #[serde(default)]
    pub disable: Vec<String>,
}

/// The chosen components and why.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Selection {
    /// In install order.
    #[serde(default)]
    pub components: Vec<Chosen>,
    /// Each component taken out because it cannot run here, by id.
    #[serde(default)]
    pub left_out: BTreeMap<String, LeftOut>,
    /// In any order.
    #[serde(default)]
    pub warnings: Vec<Warning>,
}

/// A component to install.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Chosen {
    /// Its id.
    pub id: String,
    /// Why it is installed.
    pub why: Why,
}

/// Why a component is installed.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum Why {
    /// It is recommended on the profile, or the installer adds it.
    Default,
    /// The person enables it.
    Enabled,
    /// A chosen component needs it.
    Dependency,
}

/// Why a chosen component is taken out.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "reason", rename_all = "kebab-case")]
pub enum LeftOut {
    /// Its `distributions` do not list the cluster's.
    Distributions {
        /// What it runs on.
        distributions: Vec<Distribution>,
    },
    /// It is `unsupported` on the profile.
    Unsupported {
        /// The profile.
        profile: Profile,
    },
}

/// Something the person should know about the selection.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "kebab-case")]
pub enum Warning {
    /// A component to install did not pass its test on the distribution and platform.
    Untested {
        /// The component.
        component: String,
        /// The distribution.
        distribution: Distribution,
    },
    /// An enabled component is left out.
    LeftOut {
        /// The component.
        component: String,
    },
}

/// Why nothing is installed.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, thiserror::Error)]
#[serde(tag = "kind", rename_all = "kebab-case")]
pub enum SelectError {
    /// A name is no component id or capability.
    #[error("{name} is no component or capability in the catalog")]
    Unknown {
        /// The name.
        name: String,
    },
    /// A chosen component needs one that is disabled or left out.
    #[error("{component} needs {dependency}, which is disabled or cannot run here")]
    NeedsDisabled {
        /// The chosen component.
        component: String,
        /// What it needs.
        dependency: String,
    },
}

/// Chooses the components to install from `candidates` for `config`.
///
/// # Errors
///
/// A name is unknown, or a chosen component needs one that is disabled or left out.
pub fn select(candidates: &[Candidate], config: &Config) -> Result<Selection, SelectError> {
    let by_id: BTreeMap<&str, &Candidate> = candidates.iter().map(|c| (c.id.as_str(), c)).collect();
    let mut chosen: BTreeMap<&str, Why> = by_id
        .iter()
        .filter(|(_, c)| c.defaults.mode(config.profile) == Mode::Recommended)
        .map(|(id, _)| (*id, Why::Default))
        .collect();
    for name in &config.add {
        chosen
            .entry(resolve(&by_id, name, config)?)
            .or_insert(Why::Default);
    }
    for name in &config.enable {
        chosen.insert(resolve(&by_id, name, config)?, Why::Enabled);
    }

    let mut disabled: BTreeSet<&str> = BTreeSet::new();
    for name in &config.disable {
        disabled.extend(named(&by_id, name)?);
    }
    chosen.retain(|id, _| !disabled.contains(id));

    let mut selection = Selection::default();
    chosen.retain(|id, why| {
        let Some(reason) = by_id.get(id).and_then(|c| c.left_out(config)) else {
            return true;
        };
        if *why == Why::Enabled {
            selection.warnings.push(Warning::LeftOut {
                component: (*id).to_owned(),
            });
        }
        selection.left_out.insert((*id).to_owned(), reason);
        false
    });

    for (id, candidate) in chosen.keys().filter_map(|id| Some((*id, by_id.get(id)?))) {
        let alone = [candidate.id.clone()];
        let order = candidate
            .install_order
            .get(config.profile)
            .unwrap_or(&alone);
        for dependency in order {
            if selection.components.iter().any(|c| c.id == *dependency) {
                continue;
            }
            let needed = by_id.get(dependency.as_str());
            if dependency != id
                && (disabled.contains(dependency.as_str())
                    || needed.is_none_or(|n| n.left_out(config).is_some()))
            {
                return Err(SelectError::NeedsDisabled {
                    component: id.to_owned(),
                    dependency: dependency.clone(),
                });
            }
            let why = chosen
                .get(dependency.as_str())
                .copied()
                .unwrap_or(Why::Dependency);
            selection.components.push(Chosen {
                id: dependency.clone(),
                why,
            });
        }
    }

    for chosen in &selection.components {
        if by_id
            .get(chosen.id.as_str())
            .is_some_and(|c| !c.tested(config))
        {
            selection.warnings.push(Warning::Untested {
                component: chosen.id.clone(),
                distribution: config.distribution,
            });
        }
    }
    Ok(selection)
}

/// The one component an `add` or `enable` name names: the id, or the best of a capability's
/// components that are not `unsupported` on the profile (tested first, then recommended, then
/// id). When all are `unsupported`, the first by id, which is then left out.
fn resolve<'a>(
    by_id: &BTreeMap<&'a str, &'a Candidate>,
    name: &str,
    config: &Config,
) -> Result<&'a str, SelectError> {
    if let Some((id, _)) = by_id.get_key_value(name) {
        return Ok(id);
    }
    let providing: Vec<(&'a str, &'a Candidate)> = by_id
        .iter()
        .filter(|(_, c)| c.capability() == Some(name))
        .map(|(id, c)| (*id, *c))
        .collect();
    let supported = providing
        .iter()
        .filter(|(_, c)| c.defaults.mode(config.profile) != Mode::Unsupported)
        .min_by_key(|(id, c)| {
            (
                !c.tested(config),
                c.defaults.mode(config.profile) != Mode::Recommended,
                *id,
            )
        });
    supported
        .or_else(|| providing.first())
        .map(|(id, _)| *id)
        .ok_or_else(|| SelectError::Unknown {
            name: name.to_owned(),
        })
}

/// Every component a `disable` name names: the id, or every component of the capability.
fn named<'a>(
    by_id: &BTreeMap<&'a str, &'a Candidate>,
    name: &str,
) -> Result<Vec<&'a str>, SelectError> {
    if let Some((id, _)) = by_id.get_key_value(name) {
        return Ok(vec![*id]);
    }
    let ids: Vec<&'a str> = by_id
        .values()
        .filter(|c| c.capability() == Some(name))
        .map(|c| c.id.as_str())
        .collect();
    if ids.is_empty() {
        return Err(SelectError::Unknown {
            name: name.to_owned(),
        });
    }
    Ok(ids)
}

/// A case of `conformance/select/`: a small catalog, a configuration and what an installer
/// chooses.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SelectCase {
    /// The rule the case checks.
    pub description: String,
    /// The catalog.
    pub components: Vec<Candidate>,
    /// The configuration.
    pub config: Config,
    /// The selection, or the error.
    pub want: Expected,
}

/// What a case expects. A list or map left out is empty.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Expected {
    /// In install order.
    #[serde(default)]
    pub components: Vec<Chosen>,
    /// By id.
    #[serde(default)]
    pub left_out: BTreeMap<String, LeftOut>,
    /// In any order.
    #[serde(default)]
    pub warnings: Vec<Warning>,
    /// The error, instead of a selection.
    #[serde(default)]
    pub error: Option<SelectError>,
}
