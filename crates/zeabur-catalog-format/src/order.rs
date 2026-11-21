//! Install order [installers]: what installing a component installs on a profile.
//!
//! [installers]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/installers.mdx#install-order

use std::collections::BTreeMap;

use crate::common::{Mode, Profile, Reference};
use crate::component::Component;

/// A `requires` entry that names nothing, or a loop of them.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum OrderError {
    /// No component has the id.
    #[error("{by} requires the unknown component {id}")]
    UnknownComponent {
        /// The component whose `requires` names it.
        by: String,
        /// The id.
        id: String,
    },
    /// No component provides the capability on the profile.
    #[error("{by} requires {capability}, which nothing provides on {profile}")]
    NoProvider {
        /// The component whose `requires` names it.
        by: String,
        /// The capability.
        capability: String,
        /// The profile.
        profile: Profile,
    },
    /// The requirements loop.
    #[error("dependency cycle through {0}")]
    Cycle(String),
}

/// The component a `requires` entry names on `profile`: the id, or for a capability the first
/// component in id order that provides it and is `recommended`, else the first that is
/// `optional`.
///
/// # Errors
///
/// It names nothing. `by` names the component in the error.
pub fn provider<'a, C: AsRef<Component>>(
    components: &'a BTreeMap<String, C>,
    reference: &Reference,
    profile: Profile,
    by: &str,
) -> Result<&'a str, OrderError> {
    match reference {
        Reference::Component(id) => components
            .get_key_value(id)
            .map(|(id, _)| id.as_str())
            .ok_or_else(|| OrderError::UnknownComponent {
                by: by.to_owned(),
                id: id.clone(),
            }),
        Reference::Capability(capability) => {
            let providing = || {
                components
                    .iter()
                    .filter(|(_, c)| c.as_ref().capability() == Some(capability.as_str()))
            };
            providing()
                .find(|(_, c)| c.as_ref().mode(profile) == Mode::Recommended)
                .or_else(|| providing().find(|(_, c)| c.as_ref().mode(profile) == Mode::Optional))
                .map(|(id, _)| id.as_str())
                .ok_or_else(|| OrderError::NoProvider {
                    by: by.to_owned(),
                    capability: capability.clone(),
                    profile,
                })
        }
    }
}

/// The components that installing `id` installs on `profile`, each after what it requires, with
/// `id` last. `None` when `id` is unknown or `unsupported` on the profile.
///
/// # Errors
///
/// A `requires` entry names nothing, or the requirements loop.
pub fn install_order<C: AsRef<Component>>(
    components: &BTreeMap<String, C>,
    id: &str,
    profile: Profile,
) -> Result<Option<Vec<String>>, OrderError> {
    if components
        .get(id)
        .is_none_or(|c| c.as_ref().mode(profile) == Mode::Unsupported)
    {
        return Ok(None);
    }
    let mut order = Vec::new();
    let mut visiting = Vec::new();
    visit(components, id, profile, &mut visiting, &mut order)?;
    Ok(Some(order))
}

fn visit<C: AsRef<Component>>(
    components: &BTreeMap<String, C>,
    id: &str,
    profile: Profile,
    visiting: &mut Vec<String>,
    order: &mut Vec<String>,
) -> Result<(), OrderError> {
    if order.iter().any(|o| o == id) {
        return Ok(());
    }
    if visiting.iter().any(|v| v == id) {
        return Err(OrderError::Cycle(id.to_owned()));
    }
    visiting.push(id.to_owned());
    let requires = components
        .get(id)
        .map_or(&[][..], |c| c.as_ref().requires.as_slice());
    for reference in requires {
        let dependency = provider(components, reference, profile, id)?;
        visit(components, dependency, profile, visiting, order)?;
    }
    visiting.pop();
    order.push(id.to_owned());
    Ok(())
}
