//! Values that components add to releases of other components.

use std::collections::{BTreeMap, BTreeSet};

use crate::component::{Component, ReleaseRef};
use crate::values::{self, Values};

/// What a set of contributors adds to each release.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Contributions(BTreeMap<ReleaseRef, Values>);

/// A contribution that is no YAML mapping.
#[derive(Debug, thiserror::Error)]
#[error("{component} contributes to {}/{}: {message}", target.component, target.release)]
pub struct ContributionError {
    /// The contributor.
    pub component: String,
    /// The release it contributes to.
    pub target: ReleaseRef,
    /// Why.
    pub message: String,
}

impl Contributions {
    /// What `contributors` add, merged in id order, so the later id wins a key both set. An id
    /// that is not in `components` adds nothing.
    ///
    /// # Errors
    ///
    /// A contribution is no YAML mapping.
    pub fn of<'a, C: AsRef<Component>>(
        components: &BTreeMap<String, C>,
        contributors: impl IntoIterator<Item = &'a str>,
    ) -> Result<Self, ContributionError> {
        let contributors: BTreeSet<&str> = contributors.into_iter().collect();
        let mut added: BTreeMap<ReleaseRef, Values> = BTreeMap::new();
        for id in contributors {
            let Some(component) = components.get(id) else {
                continue;
            };
            for c in &component.as_ref().contributes {
                let values = values::parse(&c.values).map_err(|e| ContributionError {
                    component: id.to_owned(),
                    target: c.target.clone(),
                    message: e.to_string(),
                })?;
                values::merge(added.entry(c.target.clone()).or_default(), values);
            }
        }
        Ok(Self(added))
    }

    /// What the components before `id` in `order` add: a component's step takes values only
    /// from what is installed before it, so installing it never needs a later component.
    ///
    /// # Errors
    ///
    /// A contribution is no YAML mapping.
    pub fn before<C: AsRef<Component>>(
        components: &BTreeMap<String, C>,
        order: &[String],
        id: &str,
    ) -> Result<Self, ContributionError> {
        Self::of(
            components,
            order.iter().map(String::as_str).take_while(|o| *o != id),
        )
    }

    /// What is added to `component`'s `release`; empty when nothing is.
    #[must_use]
    pub fn to(&self, component: &str, release: &str) -> Values {
        let target = ReleaseRef {
            component: component.to_owned(),
            release: release.to_owned(),
        };
        self.0.get(&target).cloned().unwrap_or_default()
    }
}
