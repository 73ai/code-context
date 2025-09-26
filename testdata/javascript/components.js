/**
 * UI Components and Widget Classes for the JavaScript test project.
 *
 * This module provides:
 * - Base component architecture
 * - Form components and validation
 * - Data display components
 * - Interactive widgets
 * - Modal and overlay components
 * - Navigation components
 * - Custom web components
 */

import { EventEmitter } from 'events';
import { Utils } from './utils.js';

/**
 * Base Component class that other components extend
 */
export class BaseComponent extends EventEmitter {
    constructor(element, options = {}) {
        super();

        this.element = typeof element === 'string' ? document.querySelector(element) : element;
        this.options = { ...this.constructor.defaultOptions, ...options };
        this.isInitialized = false;
        this.isDestroyed = false;

        if (!this.element) {
            throw new Error('Component element not found');
        }

        this.element._component = this;
        this.init();
    }

    static get defaultOptions() {
        return {};
    }

    init() {
        if (this.isInitialized || this.isDestroyed) return;

        this.createElement();
        this.bindEvents();
        this.render();

        this.isInitialized = true;
        this.emit('initialized');
    }

    createElement() {
    }

    bindEvents() {
    }

    render() {
    }

    update(options = {}) {
        this.options = { ...this.options, ...options };
        this.render();
        this.emit('updated', this.options);
    }

    destroy() {
        if (this.isDestroyed) return;

        this.unbindEvents();
        this.removeElement();

        if (this.element._component) {
            delete this.element._component;
        }

        this.isDestroyed = true;
        this.emit('destroyed');
    }

    unbindEvents() {
    }

    removeElement() {
    }

    show() {
        this.element.style.display = '';
        this.element.classList.remove('hidden');
        this.emit('shown');
    }

    hide() {
        this.element.style.display = 'none';
        this.element.classList.add('hidden');
        this.emit('hidden');
    }

    enable() {
        this.element.removeAttribute('disabled');
        this.element.classList.remove('disabled');
        this.emit('enabled');
    }

    disable() {
        this.element.setAttribute('disabled', 'disabled');
        this.element.classList.add('disabled');
        this.emit('disabled');
    }
}

/**
 * Button component with various states and types
 */
export class Button extends BaseComponent {
    static get defaultOptions() {
        return {
            type: 'button',
            variant: 'primary',
            size: 'medium',
            disabled: false,
            loading: false,
            icon: null,
            text: 'Button'
        };
    }

    createElement() {
        if (this.element.tagName !== 'BUTTON') {
            const button = document.createElement('button');
            this.element.parentNode?.replaceChild(button, this.element);
            this.element = button;
        }
    }

    bindEvents() {
        this.clickHandler = this.handleClick.bind(this);
        this.element.addEventListener('click', this.clickHandler);
    }

    unbindEvents() {
        if (this.clickHandler) {
            this.element.removeEventListener('click', this.clickHandler);
        }
    }

    render() {
        const { type, variant, size, disabled, loading, icon, text } = this.options;

        this.element.type = type;
        this.element.className = `btn btn--${variant} btn--${size}`;
        this.element.disabled = disabled || loading;

        let content = '';

        if (loading) {
            content += '<span class="btn__spinner"></span>';
        }

        if (icon && !loading) {
            content += `<i class="btn__icon ${icon}"></i>`;
        }

        content += `<span class="btn__text">${text}</span>`;

        this.element.innerHTML = content;

        if (loading) {
            this.element.classList.add('btn--loading');
        } else {
            this.element.classList.remove('btn--loading');
        }
    }

    handleClick(event) {
        if (this.options.disabled || this.options.loading) {
            event.preventDefault();
            return;
        }

        this.emit('click', event);
    }

    setLoading(loading = true) {
        this.update({ loading });
    }

    setText(text) {
        this.update({ text });
    }
}

/**
 * Form input component with validation
 */
export class Input extends BaseComponent {
    static get defaultOptions() {
        return {
            type: 'text',
            placeholder: '',
            value: '',
            required: false,
            disabled: false,
            readonly: false,
            maxLength: null,
            pattern: null,
            validators: [],
            errorMessage: '',
            successMessage: '',
            debounceMs: 300
        };
    }

    createElement() {
        this.wrapper = document.createElement('div');
        this.wrapper.className = 'input-wrapper';

        this.input = document.createElement('input');
        this.input.className = 'input__field';

        this.errorElement = document.createElement('div');
        this.errorElement.className = 'input__error';

        this.successElement = document.createElement('div');
        this.successElement.className = 'input__success';

        this.wrapper.appendChild(this.input);
        this.wrapper.appendChild(this.errorElement);
        this.wrapper.appendChild(this.successElement);

        this.element.appendChild(this.wrapper);
    }

    bindEvents() {
        this.inputHandler = Utils.Async.debounce(this.handleInput.bind(this), this.options.debounceMs);
        this.blurHandler = this.handleBlur.bind(this);
        this.focusHandler = this.handleFocus.bind(this);

        this.input.addEventListener('input', this.inputHandler);
        this.input.addEventListener('blur', this.blurHandler);
        this.input.addEventListener('focus', this.focusHandler);
    }

    unbindEvents() {
        if (this.inputHandler) {
            this.input.removeEventListener('input', this.inputHandler);
        }
        if (this.blurHandler) {
            this.input.removeEventListener('blur', this.blurHandler);
        }
        if (this.focusHandler) {
            this.input.removeEventListener('focus', this.focusHandler);
        }
    }

    render() {
        const { type, placeholder, value, required, disabled, readonly, maxLength, pattern } = this.options;

        this.input.type = type;
        this.input.placeholder = placeholder;
        this.input.value = value;
        this.input.required = required;
        this.input.disabled = disabled;
        this.input.readOnly = readonly;

        if (maxLength) this.input.maxLength = maxLength;
        if (pattern) this.input.pattern = pattern;

        this.updateValidationState();
    }

    handleInput(event) {
        const value = event.target.value;
        this.options.value = value;

        this.validate();
        this.emit('input', { value, valid: this.isValid });
    }

    handleBlur(event) {
        this.validate();
        this.emit('blur', { value: this.getValue(), valid: this.isValid });
    }

    handleFocus(event) {
        this.clearValidationState();
        this.emit('focus', { value: this.getValue() });
    }

    validate() {
        const value = this.getValue();
        const errors = [];

        if (this.options.required && !value.trim()) {
            errors.push('This field is required');
        }

        for (const validator of this.options.validators) {
            const result = validator(value);
            if (result !== true) {
                errors.push(typeof result === 'string' ? result : 'Invalid value');
            }
        }

        this.isValid = errors.length === 0;
        this.validationErrors = errors;

        this.updateValidationState();
        return this.isValid;
    }

    updateValidationState() {
        this.wrapper.classList.remove('input--error', 'input--success');
        this.errorElement.textContent = '';
        this.successElement.textContent = '';

        if (this.validationErrors && this.validationErrors.length > 0) {
            this.wrapper.classList.add('input--error');
            this.errorElement.textContent = this.validationErrors[0];
        } else if (this.isValid && this.getValue().trim()) {
            this.wrapper.classList.add('input--success');
            this.successElement.textContent = this.options.successMessage;
        }
    }

    clearValidationState() {
        this.wrapper.classList.remove('input--error', 'input--success');
        this.errorElement.textContent = '';
        this.successElement.textContent = '';
    }

    getValue() {
        return this.input.value;
    }

    setValue(value) {
        this.input.value = value;
        this.options.value = value;
        this.validate();
    }

    focus() {
        this.input.focus();
    }

    blur() {
        this.input.blur();
    }
}

/**
 * Modal/Dialog component
 */
export class Modal extends BaseComponent {
    static get defaultOptions() {
        return {
            title: '',
            content: '',
            closable: true,
            backdrop: true,
            keyboard: true,
            size: 'medium',
            animation: 'fade',
            autoFocus: true
        };
    }

    createElement() {
        this.overlay = document.createElement('div');
        this.overlay.className = 'modal-overlay';

        this.modal = document.createElement('div');
        this.modal.className = `modal modal--${this.options.size}`;

        this.header = document.createElement('div');
        this.header.className = 'modal__header';

        this.title = document.createElement('h3');
        this.title.className = 'modal__title';

        this.closeBtn = document.createElement('button');
        this.closeBtn.className = 'modal__close';
        this.closeBtn.innerHTML = '&times;';
        this.closeBtn.setAttribute('aria-label', 'Close modal');

        this.body = document.createElement('div');
        this.body.className = 'modal__body';

        this.footer = document.createElement('div');
        this.footer.className = 'modal__footer';

        this.header.appendChild(this.title);
        if (this.options.closable) {
            this.header.appendChild(this.closeBtn);
        }

        this.modal.appendChild(this.header);
        this.modal.appendChild(this.body);
        this.modal.appendChild(this.footer);
        this.overlay.appendChild(this.modal);

        document.body.appendChild(this.overlay);
    }

    bindEvents() {
        this.closeHandler = this.handleClose.bind(this);
        this.keyHandler = this.handleKeydown.bind(this);
        this.overlayClickHandler = this.handleOverlayClick.bind(this);

        if (this.options.closable) {
            this.closeBtn.addEventListener('click', this.closeHandler);
        }

        if (this.options.keyboard) {
            document.addEventListener('keydown', this.keyHandler);
        }

        if (this.options.backdrop) {
            this.overlay.addEventListener('click', this.overlayClickHandler);
        }
    }

    unbindEvents() {
        if (this.closeHandler) {
            this.closeBtn?.removeEventListener('click', this.closeHandler);
        }
        if (this.keyHandler) {
            document.removeEventListener('keydown', this.keyHandler);
        }
        if (this.overlayClickHandler) {
            this.overlay?.removeEventListener('click', this.overlayClickHandler);
        }
    }

    render() {
        this.title.textContent = this.options.title;
        this.body.innerHTML = this.options.content;

        if (this.options.animation) {
            this.overlay.classList.add(`modal--${this.options.animation}`);
        }
    }

    handleClose() {
        this.close();
    }

    handleKeydown(event) {
        if (event.key === 'Escape' && this.isOpen) {
            this.close();
        }
    }

    handleOverlayClick(event) {
        if (event.target === this.overlay) {
            this.close();
        }
    }

    open() {
        if (this.isOpen) return;

        this.overlay.classList.add('modal--open');
        document.body.classList.add('modal-open');

        if (this.options.autoFocus) {
            const focusable = this.modal.querySelector('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
            focusable?.focus();
        }

        this.isOpen = true;
        this.emit('opened');
    }

    close() {
        if (!this.isOpen) return;

        this.overlay.classList.remove('modal--open');
        document.body.classList.remove('modal-open');

        this.isOpen = false;
        this.emit('closed');
    }

    setTitle(title) {
        this.update({ title });
    }

    setContent(content) {
        this.update({ content });
    }

    removeElement() {
        if (this.overlay && this.overlay.parentNode) {
            this.overlay.parentNode.removeChild(this.overlay);
        }
        document.body.classList.remove('modal-open');
    }
}

/**
 * Dropdown/Select component
 */
export class Dropdown extends BaseComponent {
    static get defaultOptions() {
        return {
            options: [],
            value: null,
            placeholder: 'Select an option...',
            searchable: false,
            multiple: false,
            disabled: false,
            clearable: true
        };
    }

    createElement() {
        this.dropdown = document.createElement('div');
        this.dropdown.className = 'dropdown';

        this.trigger = document.createElement('button');
        this.trigger.className = 'dropdown__trigger';
        this.trigger.type = 'button';

        this.menu = document.createElement('div');
        this.menu.className = 'dropdown__menu';

        if (this.options.searchable) {
            this.search = document.createElement('input');
            this.search.className = 'dropdown__search';
            this.search.type = 'text';
            this.search.placeholder = 'Search...';
            this.menu.appendChild(this.search);
        }

        this.list = document.createElement('ul');
        this.list.className = 'dropdown__list';
        this.menu.appendChild(this.list);

        this.dropdown.appendChild(this.trigger);
        this.dropdown.appendChild(this.menu);
        this.element.appendChild(this.dropdown);
    }

    bindEvents() {
        this.triggerHandler = this.handleTriggerClick.bind(this);
        this.documentClickHandler = this.handleDocumentClick.bind(this);
        this.keyHandler = this.handleKeydown.bind(this);

        this.trigger.addEventListener('click', this.triggerHandler);
        document.addEventListener('click', this.documentClickHandler);
        document.addEventListener('keydown', this.keyHandler);

        if (this.search) {
            this.searchHandler = Utils.Async.debounce(this.handleSearch.bind(this), 200);
            this.search.addEventListener('input', this.searchHandler);
        }
    }

    unbindEvents() {
        this.trigger?.removeEventListener('click', this.triggerHandler);
        document.removeEventListener('click', this.documentClickHandler);
        document.removeEventListener('keydown', this.keyHandler);
        this.search?.removeEventListener('input', this.searchHandler);
    }

    render() {
        this.renderTrigger();
        this.renderOptions();
    }

    renderTrigger() {
        let text = this.options.placeholder;

        if (this.selectedValues.length > 0) {
            if (this.options.multiple) {
                text = `${this.selectedValues.length} selected`;
            } else {
                const selected = this.options.options.find(opt => opt.value === this.selectedValues[0]);
                text = selected ? selected.label : this.selectedValues[0];
            }
        }

        this.trigger.innerHTML = `
            <span class="dropdown__text">${text}</span>
            <span class="dropdown__arrow">▼</span>
        `;

        this.trigger.disabled = this.options.disabled;
    }

    renderOptions() {
        this.list.innerHTML = '';

        const filteredOptions = this.searchTerm
            ? this.options.options.filter(opt =>
                opt.label.toLowerCase().includes(this.searchTerm.toLowerCase())
              )
            : this.options.options;

        filteredOptions.forEach(option => {
            const li = document.createElement('li');
            li.className = 'dropdown__item';
            li.dataset.value = option.value;

            if (this.isSelected(option.value)) {
                li.classList.add('dropdown__item--selected');
            }

            li.innerHTML = `
                <span class="dropdown__item-text">${option.label}</span>
                ${this.options.multiple ? '<span class="dropdown__check">✓</span>' : ''}
            `;

            li.addEventListener('click', () => this.selectOption(option));
            this.list.appendChild(li);
        });
    }

    handleTriggerClick(event) {
        event.stopPropagation();
        this.toggle();
    }

    handleDocumentClick(event) {
        if (!this.dropdown.contains(event.target)) {
            this.close();
        }
    }

    handleKeydown(event) {
        if (!this.isOpen) return;

        switch (event.key) {
            case 'Escape':
                this.close();
                break;
            case 'ArrowDown':
                event.preventDefault();
                this.navigateOptions(1);
                break;
            case 'ArrowUp':
                event.preventDefault();
                this.navigateOptions(-1);
                break;
            case 'Enter':
                event.preventDefault();
                this.selectFocused();
                break;
        }
    }

    handleSearch(event) {
        this.searchTerm = event.target.value;
        this.renderOptions();
    }

    open() {
        if (this.isOpen || this.options.disabled) return;

        this.dropdown.classList.add('dropdown--open');
        this.isOpen = true;

        if (this.search) {
            this.search.focus();
        }

        this.emit('opened');
    }

    close() {
        if (!this.isOpen) return;

        this.dropdown.classList.remove('dropdown--open');
        this.isOpen = false;
        this.emit('closed');
    }

    toggle() {
        this.isOpen ? this.close() : this.open();
    }

    selectOption(option) {
        if (!this.selectedValues) {
            this.selectedValues = [];
        }

        if (this.options.multiple) {
            const index = this.selectedValues.indexOf(option.value);
            if (index > -1) {
                this.selectedValues.splice(index, 1);
            } else {
                this.selectedValues.push(option.value);
            }
        } else {
            this.selectedValues = [option.value];
            this.close();
        }

        this.render();
        this.emit('change', {
            value: this.options.multiple ? this.selectedValues : this.selectedValues[0],
            option: option
        });
    }

    isSelected(value) {
        return this.selectedValues && this.selectedValues.includes(value);
    }

    getValue() {
        if (!this.selectedValues || this.selectedValues.length === 0) {
            return this.options.multiple ? [] : null;
        }
        return this.options.multiple ? this.selectedValues : this.selectedValues[0];
    }

    setValue(value) {
        if (Array.isArray(value)) {
            this.selectedValues = [...value];
        } else {
            this.selectedValues = value !== null ? [value] : [];
        }
        this.render();
    }
}

/**
 * Tab component for tabbed interfaces
 */
export class Tabs extends BaseComponent {
    static get defaultOptions() {
        return {
            activeIndex: 0,
            orientation: 'horizontal',
            activateOnFocus: false
        };
    }

    createElement() {
        this.tabList = this.element.querySelector('[role="tablist"]') || this.createTabList();
        this.tabs = Array.from(this.tabList.querySelectorAll('[role="tab"]'));
        this.panels = Array.from(this.element.querySelectorAll('[role="tabpanel"]'));

        this.setupAccessibility();
    }

    createTabList() {
        const existingTabs = this.element.querySelectorAll('.tab');
        if (existingTabs.length === 0) return null;

        const tabList = document.createElement('div');
        tabList.setAttribute('role', 'tablist');
        tabList.className = 'tab-list';

        existingTabs.forEach((tab, index) => {
            tab.setAttribute('role', 'tab');
            tab.setAttribute('tabindex', index === this.options.activeIndex ? '0' : '-1');
            tab.id = tab.id || `tab-${index}`;
            tabList.appendChild(tab);
        });

        this.element.insertBefore(tabList, this.element.firstChild);
        return tabList;
    }

    setupAccessibility() {
        this.tabs.forEach((tab, index) => {
            tab.setAttribute('aria-controls', `panel-${index}`);
            tab.setAttribute('aria-selected', index === this.options.activeIndex ? 'true' : 'false');
        });

        this.panels.forEach((panel, index) => {
            panel.setAttribute('role', 'tabpanel');
            panel.setAttribute('aria-labelledby', `tab-${index}`);
            panel.id = panel.id || `panel-${index}`;
            panel.hidden = index !== this.options.activeIndex;
        });
    }

    bindEvents() {
        this.tabs.forEach((tab, index) => {
            tab.addEventListener('click', () => this.activateTab(index));
            tab.addEventListener('keydown', (e) => this.handleKeydown(e, index));

            if (this.options.activateOnFocus) {
                tab.addEventListener('focus', () => this.activateTab(index));
            }
        });
    }

    handleKeydown(event, currentIndex) {
        let newIndex;

        switch (event.key) {
            case 'ArrowLeft':
            case 'ArrowUp':
                event.preventDefault();
                newIndex = currentIndex > 0 ? currentIndex - 1 : this.tabs.length - 1;
                break;
            case 'ArrowRight':
            case 'ArrowDown':
                event.preventDefault();
                newIndex = currentIndex < this.tabs.length - 1 ? currentIndex + 1 : 0;
                break;
            case 'Home':
                event.preventDefault();
                newIndex = 0;
                break;
            case 'End':
                event.preventDefault();
                newIndex = this.tabs.length - 1;
                break;
            default:
                return;
        }

        this.tabs[newIndex].focus();
        if (this.options.activateOnFocus) {
            this.activateTab(newIndex);
        }
    }

    activateTab(index) {
        if (index === this.activeIndex) return;

        if (this.activeIndex !== undefined) {
            this.tabs[this.activeIndex].setAttribute('aria-selected', 'false');
            this.tabs[this.activeIndex].setAttribute('tabindex', '-1');
            this.tabs[this.activeIndex].classList.remove('tab--active');
            this.panels[this.activeIndex].hidden = true;
        }

        this.activeIndex = index;
        this.tabs[index].setAttribute('aria-selected', 'true');
        this.tabs[index].setAttribute('tabindex', '0');
        this.tabs[index].classList.add('tab--active');
        this.panels[index].hidden = false;

        this.emit('change', { index, tab: this.tabs[index], panel: this.panels[index] });
    }

    getActiveIndex() {
        return this.activeIndex;
    }

    getActiveTab() {
        return this.tabs[this.activeIndex];
    }

    getActivePanel() {
        return this.panels[this.activeIndex];
    }
}

/**
 * Component factory and registry
 */
export class ComponentRegistry {
    constructor() {
        this.components = new Map();
        this.instances = new WeakMap();
    }

    register(name, componentClass) {
        this.components.set(name, componentClass);
    }

    create(name, element, options = {}) {
        const ComponentClass = this.components.get(name);
        if (!ComponentClass) {
            throw new Error(`Component '${name}' not found`);
        }

        const instance = new ComponentClass(element, options);
        this.instances.set(element, instance);
        return instance;
    }

    get(element) {
        return this.instances.get(element) || element._component;
    }

    autoInit(selector = '[data-component]') {
        const elements = document.querySelectorAll(selector);
        const instances = [];

        elements.forEach(element => {
            const componentName = element.dataset.component;
            const options = element.dataset.options ? JSON.parse(element.dataset.options) : {};

            try {
                const instance = this.create(componentName, element, options);
                instances.push(instance);
            } catch (error) {
                console.error(`Failed to initialize component '${componentName}':`, error);
            }
        });

        return instances;
    }
}

export const registry = new ComponentRegistry();
registry.register('button', Button);
registry.register('input', Input);
registry.register('modal', Modal);
registry.register('dropdown', Dropdown);
registry.register('tabs', Tabs);

if (typeof document !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => registry.autoInit());
    } else {
        registry.autoInit();
    }
}

export {
    BaseComponent,
    Button,
    Input,
    Modal,
    Dropdown,
    Tabs,
    ComponentRegistry
};

export default {
    BaseComponent,
    Button,
    Input,
    Modal,
    Dropdown,
    Tabs,
    ComponentRegistry,
    registry
};